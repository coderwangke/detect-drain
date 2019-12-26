package check

import (
	"fmt"
	"github.com/coderwangke/detect-drain/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/klog"
	"net"
)

const NONE_RESOURCE = "none"

type NodeDetail struct {
	NodeName         string
	MaxPods          uint
	CurrentPods      string
	Eips             int
	GpuNode          bool
	Schedule         bool
	CpuAllocatable   string
	MemAllocatable   string
	CpuAllocated     string
	MemAllocated     string
	KubeletVersion   string
	KubeproxyVersion string
	KernelVersion    string
}

type DetectNode struct {
	DrainNode   string
	Client      *utils.KubeCient
	NodeDetails []NodeDetail
}

func NewDetectNode(drainNode string, client *utils.KubeCient) *DetectNode {
	return &DetectNode{
		DrainNode:   drainNode,
		Client:      client,
		NodeDetails: []NodeDetail{},
	}
}

func (dn *DetectNode) Detect() error {
	nodeClient := dn.Client.ClientSet.CoreV1().Nodes()
	// get drain node
	nodeList, err := nodeClient.List(metav1.ListOptions{})

	if err != nil {
		klog.Errorf("Failed to list node: %v", err)
		return err
	}

	for _, n := range nodeList.Items {
		cpuReqs, _, memReqs, _ := dn.getNodeResource(&n)
		currentPods := dn.getNodeNonTerminatedPodsListNumber(&n)
		nd := NodeDetail{
			NodeName:         n.Name,
			MaxPods:          getMaxPods(n.Spec.PodCIDR),
			CurrentPods:      currentPods,
			Eips:             0,
			GpuNode:          false,
			Schedule:         schedule(&n),
			CpuAllocatable:   n.Status.Allocatable.Cpu().String(),
			MemAllocatable:   n.Status.Allocatable.Memory().String(),
			CpuAllocated:     cpuReqs,
			MemAllocated:     memReqs,
			KubeletVersion:   n.Status.NodeInfo.KubeletVersion,
			KubeproxyVersion: n.Status.NodeInfo.KubeProxyVersion,
			KernelVersion:    n.Status.NodeInfo.KernelVersion,
		}

		dn.NodeDetails = append(dn.NodeDetails, nd)
	}

	return nil
}

func getMaxPods(cidr string) uint {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		klog.Errorf("Failed to parseCidr %s: %v", cidr, err)
		return 0
	}
	mask, _ := ipNet.Mask.Size()

	cidrIpNum := uint(0)
	var i = uint(32 - mask - 1)
	for ; i >= 1; i-- {
		cidrIpNum += 1 << i
	}

	return cidrIpNum
}

func getEips() int {

	return 0
}

func gpuNode() bool {

	return false
}

func schedule(node *corev1.Node) bool {
	return !node.Spec.Unschedulable
}

// TODO get nodeNonTerminatedPodsList number

func (dn *DetectNode) getNodeNonTerminatedPodsListNumber(node *corev1.Node) string {
	pods := dn.nodeNonTerminatedPodsList(node)
	if pods == nil {
		return NONE_RESOURCE
	}

	return fmt.Sprintf("%d", len(pods.Items))

}

func (dn *DetectNode) getNodeResource(node *corev1.Node) (string, string, string, string) {
	pods := dn.nodeNonTerminatedPodsList(node)
	if pods == nil {
		return NONE_RESOURCE, NONE_RESOURCE, NONE_RESOURCE, NONE_RESOURCE
	}
	reqs, limits := getPodsTotalRequestsAndLimits(pods)
	cpuReqs, cpuLimits, memoryReqs, memoryLimits :=
		reqs[corev1.ResourceCPU], limits[corev1.ResourceCPU], reqs[corev1.ResourceMemory], limits[corev1.ResourceMemory]

	return cpuReqs.String(), cpuLimits.String(), memoryReqs.String(), memoryLimits.String()
}

func (dn *DetectNode) nodeNonTerminatedPodsList(node *corev1.Node) *corev1.PodList {
	podClient := dn.Client.ClientSet.CoreV1().Pods("")
	fieldSelector, err := fields.ParseSelector("spec.nodeName=" + node.Name + ",status.phase!=" + string(corev1.PodSucceeded) + ",status.phase!=" + string(corev1.PodFailed))
	if err != nil {
		klog.Errorf("Failed to parseSelector: %v", err)
		return nil
	}

	nodeNonTerminatedPodsList, err := podClient.List(metav1.ListOptions{
		FieldSelector: fieldSelector.String(),
	})
	if err != nil {
		klog.Errorf("Failed to list pods of node %s: %v", node.Name, err)
		return nil
	}

	return nodeNonTerminatedPodsList
}

func getPodsTotalRequestsAndLimits(podList *corev1.PodList) (reqs map[corev1.ResourceName]resource.Quantity, limits map[corev1.ResourceName]resource.Quantity) {
	reqs, limits = map[corev1.ResourceName]resource.Quantity{}, map[corev1.ResourceName]resource.Quantity{}
	for _, pod := range podList.Items {
		podReqs, podLimits := utils.PodRequestsAndLimits(&pod)
		for podReqName, podReqValue := range podReqs {
			if value, ok := reqs[podReqName]; !ok {
				reqs[podReqName] = podReqValue.DeepCopy()
			} else {
				value.Add(podReqValue)
				reqs[podReqName] = value
			}
		}
		for podLimitName, podLimitValue := range podLimits {
			if value, ok := limits[podLimitName]; !ok {
				limits[podLimitName] = podLimitValue.DeepCopy()
			} else {
				value.Add(podLimitValue)
				limits[podLimitName] = value
			}
		}
	}
	return
}
