package check

import (
	"fmt"
	"github.com/coderwangke/detect-drain/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/klog"
)

type PodDetail struct {
	PodName      string
	Namespace    string
	OwnerRef     string
	OwnerRefKind string
	HostPath     bool
	NodeName     string
	CpuRequest   string
	MemRequest   string
	CpuLimit     string
	MemLimit     string
}

//type ResourceRef struct {
//	OwnerRef       string
//	OwnerRefKind   string
//	PodDetails     []PodDetail
//	AllocatedNodes []string
//}

type DetectNodePod struct {
	DrainNode           string
	Client              *utils.KubeCient
	PodDetails          map[string][]PodDetail
	StsPodDetails       map[string][]PodDetail
	DaemonSetPodDetails map[string][]PodDetail
	IsolatedPods        []PodDetail
}

func NewDetectNodePod(node string, client *utils.KubeCient) *DetectNodePod {
	return &DetectNodePod{
		DrainNode:           node,
		Client:              client,
		PodDetails:          make(map[string][]PodDetail),
		StsPodDetails:       make(map[string][]PodDetail),
		DaemonSetPodDetails: make(map[string][]PodDetail),
		IsolatedPods:        []PodDetail{},
	}
}

func (dbp *DetectNodePod) Detect() error {
	fmt.Println("starting detect drain node pods...")
	// get all pods running in drain node
	podClient := dbp.Client.ClientSet.CoreV1().Pods("")
	fieldSelector, err := fields.ParseSelector("spec.nodeName=" + dbp.DrainNode + ",status.phase!=" + string(corev1.PodSucceeded) + ",status.phase!=" + string(corev1.PodFailed))
	if err != nil {
		return err
	}

	nodeNonTerminatedPodsList, err := podClient.List(metav1.ListOptions{
		FieldSelector: fieldSelector.String(),
	})

	if err != nil {
		klog.Errorf("Failed to list pods: %v", err)
	}

	for _, pod := range nodeNonTerminatedPodsList.Items {
		if pod.OwnerReferences != nil {
			switch pod.OwnerReferences[0].Kind {
			case REPLICASET_WORKLOAD:
				var ownerRef string
				var ownerRefKind string
				var ns = pod.Namespace
				rsName := pod.OwnerReferences[0].Name
				deploy := dbp.getDeployment(rsName, ns)
				if deploy != nil {
					ownerRef = deploy.Name
					ownerRefKind = "Deployment"
					//fmt.Printf("Get deployment %s", deploy.Name)
				} else {
					ownerRef = rsName
					ownerRefKind = "ReplicaSet"
					//fmt.Printf("Get replicaSets %s", rsName)
				}

				cpuReq, cpuLimit, memReq, memLimit := getPodRequest(&pod)

				pd := PodDetail{
					PodName:      pod.Name,
					Namespace:    ns,
					OwnerRef:     ownerRef,
					OwnerRefKind: ownerRefKind,
					HostPath:     isHostPath(&pod),
					CpuRequest:   cpuReq,
					MemRequest:   memReq,
					CpuLimit:     cpuLimit,
					MemLimit:     memLimit,
				}

				if _, ok := dbp.PodDetails[ownerRef]; ok {
					dbp.PodDetails[ownerRef] = append(dbp.PodDetails[ownerRef], pd)
				} else {
					dbp.PodDetails[ownerRef] = []PodDetail{pd}
				}
			case STATEFULSET_WORKLOAD:
				stsName := pod.OwnerReferences[0].Name
				stsNamespace := pod.Namespace
				//sts := dbp.getStatefulSet(stsName, stsNamespace)

				cpuReq, cpuLimit, memReq, memLimit := getPodRequest(&pod)

				pd := PodDetail{
					PodName:      pod.Name,
					Namespace:    stsNamespace,
					OwnerRef:     stsName,
					OwnerRefKind: STATEFULSET_WORKLOAD,
					HostPath:     isHostPath(&pod),
					CpuRequest:   cpuReq,
					MemRequest:   memReq,
					CpuLimit:     cpuLimit,
					MemLimit:     memLimit,
				}
				if _, ok := dbp.StsPodDetails[stsName]; ok {
					dbp.StsPodDetails[stsName] = append(dbp.StsPodDetails[stsName], pd)
				} else {
					dbp.StsPodDetails[stsName] = []PodDetail{pd}
				}
			case DAEMONSET_WORKLOAD:
				dsName := pod.OwnerReferences[0].Name
				dsNamespace := pod.Namespace

				cpuReq, cpuLimit, memReq, memLimit := getPodRequest(&pod)

				pd := PodDetail{
					PodName:      pod.Name,
					Namespace:    dsNamespace,
					OwnerRef:     dsName,
					OwnerRefKind: DAEMONSET_WORKLOAD,
					HostPath:     isHostPath(&pod),
					CpuRequest:   cpuReq,
					MemRequest:   memReq,
					CpuLimit:     cpuLimit,
					MemLimit:     memLimit,
				}
				if _, ok := dbp.DaemonSetPodDetails[dsName]; ok {
					dbp.DaemonSetPodDetails[dsName] = append(dbp.DaemonSetPodDetails[dsName], pd)
				} else {
					dbp.DaemonSetPodDetails[dsName] = []PodDetail{pd}
				}
			default:
				klog.Errorf("Unknown workload kind: %s\n", pod.OwnerReferences[0].Kind)
			}
		} else {
			// isolated pod
			cpuReq, cpuLimit, memReq, memLimit := getPodRequest(&pod)
			pd := PodDetail{
				PodName:    pod.Name,
				Namespace:  pod.Namespace,
				HostPath:   isHostPath(&pod),
				CpuRequest: cpuReq,
				MemRequest: memReq,
				CpuLimit:   cpuLimit,
				MemLimit:   memLimit,
			}
			dbp.IsolatedPods = append(dbp.IsolatedPods, pd)
		}
	}

	return nil
}

func (dbp *DetectNodePod) getDeployment(rsName, ns string) *appsv1.Deployment {
	var deployName string
	// get rs
	rsClient := dbp.Client.ClientSet.AppsV1().ReplicaSets(ns)
	rs, err := rsClient.Get(rsName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed to get replicaSets: %v", err)
		return nil
	}

	if rs.OwnerReferences != nil {
		if rs.OwnerReferences[0].Kind == "Deployment" {
			deployName = rs.OwnerReferences[0].Name
		} else {
			return nil
		}
	}

	deployClient := dbp.Client.ClientSet.AppsV1().Deployments(ns)
	deploy, err := deployClient.Get(deployName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed to get deployment: %v", err)
		return nil
	}

	return deploy
}

//func (dbp *DetectNodePod) getStatefulSet(name, ns string) *appsv1.StatefulSet {
//	// get sts
//	stsClient := dbp.Client.ClientSet.AppsV1().StatefulSets(ns)
//	sts, err := stsClient.Get(name, metav1.GetOptions{})
//	if err != nil {
//		klog.Errorf("Failed to get statefulSet: %v", err)
//		return nil
//	}
//
//	return sts
//}

func isHostPath(pod *corev1.Pod) bool {
	for _, volume := range pod.Spec.Volumes {
		if volume.HostPath != nil {
			return true
		}
	}
	return false
}

func getPodRequest(pod *corev1.Pod) (string, string, string, string) {
	req, limit := utils.PodRequestsAndLimits(pod)
	cpuReq, cpuLimit, memoryReq, memoryLimit := req[corev1.ResourceCPU], limit[corev1.ResourceCPU], req[corev1.ResourceMemory], limit[corev1.ResourceMemory]

	return cpuReq.String(), cpuLimit.String(), memoryReq.String(), memoryLimit.String()
}
