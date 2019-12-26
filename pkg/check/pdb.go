package check

import (
	"fmt"
	"github.com/coderwangke/detect-drain/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog"
	"strings"
)

const (
	STATEFULSET_WORKLOAD = "StatefulSet"
	DEPLOYMENT_WORKLOAD  = "Deployment"
	REPLICASET_WORKLOAD  = "ReplicaSet"
	ZERO_AVALILABLE      = "0"
	DAEMONSET_WORKLOAD = "DaemonSet"
)

type PdbDetail struct {
	PdbName           string
	PdbNamespace      string
	PdbMinAvailable   string
	PdbMaxUnavailable string
	PdbAllowed        int32
	PodDetails        []PodDetail
}

type DetectPdb struct {
	Client     *utils.KubeCient
	PdbDetails []PdbDetail
}

func NewDetectPdb(client *utils.KubeCient) *DetectPdb {
	return &DetectPdb{
		Client:     client,
		PdbDetails: []PdbDetail{},
	}
}

func (dp *DetectPdb) Detect() error {
	fmt.Println()
	pdbClient := dp.Client.ClientSet.PolicyV1beta1().PodDisruptionBudgets("")

	pdbList, err := pdbClient.List(metav1.ListOptions{})
	if err != nil {
		klog.Errorf("Failed to list pdb: %v", err)
		return err
	}

	for _, pdb := range pdbList.Items {
		pdbde := PdbDetail{
			PdbName:           pdb.Name,
			PdbNamespace:      pdb.Namespace,
			PdbMinAvailable:   getMinAvaOrMaxUnAva(pdb.Spec.MinAvailable),
			PdbMaxUnavailable: getMinAvaOrMaxUnAva(pdb.Spec.MaxUnavailable),
			PdbAllowed:        pdb.Status.PodDisruptionsAllowed,
		}

		pdbde.PodDetails = dp.getSelectedPods(pdb.Namespace, pdb.Spec.Selector)

		dp.PdbDetails = append(dp.PdbDetails, pdbde)
	}
	return nil
}

func getMinAvaOrMaxUnAva(num *intstr.IntOrString) string {
	if num == nil {
		return ZERO_AVALILABLE
	} else {
		return num.String()
	}
}

func (dp *DetectPdb) getSelectedPods(ns string, selector *metav1.LabelSelector) []PodDetail {
	var podDetails = []PodDetail{}

	selectorString := getSelectorString(selector)
	podClient := dp.Client.ClientSet.CoreV1().Pods(ns)
	podList, err := podClient.List(metav1.ListOptions{LabelSelector: selectorString})
	if err != nil {
		klog.Errorf("DetectPdb: Failed to list pod: %v", err)
		return podDetails
	}

	for _, pod := range podList.Items {
		if pod.OwnerReferences != nil {
			switch pod.OwnerReferences[0].Kind {
			case REPLICASET_WORKLOAD:
				// ReplicaSet or Deployment
				var ownerRef string
				var ownerRefKind string
				var ns = pod.Namespace
				rsName := pod.OwnerReferences[0].Name
				deploy := dp.getDeployment(rsName, ns)
				if deploy != nil {
					ownerRef = deploy.Name
					ownerRefKind = DEPLOYMENT_WORKLOAD
					//fmt.Printf("Get deployment %s", deploy.Name)
				} else {
					ownerRef = rsName
					ownerRefKind = REPLICASET_WORKLOAD
					//fmt.Printf("Get replicaSets %s", rsName)
				}

				pd := PodDetail{
					PodName:      pod.Name,
					Namespace:    ns,
					OwnerRef:     ownerRef,
					OwnerRefKind: ownerRefKind,
					NodeName:     pod.Spec.NodeName,
				}

				podDetails = append(podDetails, pd)
			case STATEFULSET_WORKLOAD:
				// StatefulSet
				stsName := pod.OwnerReferences[0].Name
				//sts := dp.getStatefulSet(stsName, pod.Namespace)
				pd := PodDetail{
					PodName:      pod.Name,
					Namespace:    pod.Namespace,
					OwnerRef:     stsName,
					OwnerRefKind: STATEFULSET_WORKLOAD,
					NodeName:     pod.Spec.NodeName,
				}

				podDetails = append(podDetails, pd)
			default:
				klog.Errorf("Unknown workload kind: %s\n", pod.OwnerReferences[0].Kind)
			}
		} else {
			// isolated pod
			pd := PodDetail{
				PodName:      pod.Name,
				Namespace:    pod.Namespace,
				OwnerRef:     "",
				OwnerRefKind: "",
				NodeName:     pod.Spec.NodeName,
			}

			podDetails = append(podDetails, pd)
		}
	}

	return podDetails
}

func (dp *DetectPdb) getDeployment(rsName, ns string) *appsv1.Deployment {
	var deployName string
	// get rs
	rsClient := dp.Client.ClientSet.AppsV1().ReplicaSets(ns)
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

	deployClient := dp.Client.ClientSet.AppsV1().Deployments(ns)
	deploy, err := deployClient.Get(deployName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed to get deployment: %v", err)
		return nil
	}

	return deploy
}

//func (dp *DetectPdb) getStatefulSet(stsName, ns string) *appsv1.StatefulSet {
//	// get sts
//	stsClient := dp.Client.ClientSet.AppsV1().StatefulSets(ns)
//	sts, err := stsClient.Get(stsName, metav1.GetOptions{})
//	if err != nil {
//		klog.Errorf("Failed to get statefulSet: %v", err)
//		return nil
//	}
//
//	return sts
//}

func getSelectorString(selector *metav1.LabelSelector) string {
	labels := selector.MatchLabels
	var selectorLabels = make([]string, 0, len(labels))
	for key, value := range labels {
		selectorLabels = append(selectorLabels, fmt.Sprintf("%s=%s", key, value))
	}
	selectorString := strings.Join(selectorLabels, ",")
	return selectorString
}
