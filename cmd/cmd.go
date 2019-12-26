package cmd

import (
	"fmt"
	"github.com/coderwangke/detect-drain/pkg/check"
	"github.com/coderwangke/detect-drain/pkg/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"io"
	"os"
)

const programeName = "detectDrain"

type DetectDrainCmd struct {
	out        io.Writer
	drainNode  string
	kubeconfig string
}

func NewDetectDrainCmd() *cobra.Command {
	ddCmd := DetectDrainCmd{
		out: os.Stdout,
	}
	cmd := &cobra.Command{
		Use:  programeName,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ddCmd.drainNode = args[0]
			detect, err := ddCmd.run()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			} else {
				fmt.Fprintf(ddCmd.out, "%s", detect)
			}
		},
	}

	fs := cmd.PersistentFlags()
	ddCmd.addFlags(fs)

	return cmd
}

func (dd *DetectDrainCmd) addFlags(fs *pflag.FlagSet) {
	fs.StringVar(&dd.kubeconfig, "kube-config", "/root/.kube/config", "")
}

func (dd *DetectDrainCmd) run() (string, error) {
	kubeClient, err := utils.NewKubeClient(dd.kubeconfig)
	if err != nil {
		return "", err
	}

	dnpClient := check.NewDetectNodePod(dd.drainNode, kubeClient)
	err = dnpClient.Detect()
	if err != nil {
		return "", err
	}

	dnClient := check.NewDetectNode(dd.drainNode, kubeClient)
	err = dnClient.Detect()
	if err != nil {
		return "", nil
	}

	pdbClient := check.NewDetectPdb(kubeClient)
	err = pdbClient.Detect()
	if err != nil {
		return "", err
	}

	var podDetails = dnpClient.PodDetails
	var stsPodDetals = dnpClient.StsPodDetails
	var isPods = dnpClient.IsolatedPods
	var nodeDetails = dnClient.NodeDetails
	var pdbDetails = pdbClient.PdbDetails

	return utils.TabbedString(func(out io.Writer) error {
		printer := utils.New(out)
		if len(podDetails) == 0 {
			printer.Write(0, "ReplicaSetPods:\t <none>\n")
		} else {
			printer.Write(0, "ReplicaSetPods:\n")
			printer.Write(1, "owner\townerKind\tpodName\tnamespace\thasHostPath\tcpuReq\tcpuLimit\tmemReq\tmemLimit\n")
			for _, pods := range podDetails {
				for _, pod := range pods {
					printer.Write(1, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
						pod.OwnerRef, pod.OwnerRefKind, pod.PodName, pod.Namespace, fmt.Sprintf("%v", pod.HostPath == true), pod.CpuRequest, pod.CpuLimit, pod.MemRequest, pod.MemLimit)
				}
			}
		}

		if len(stsPodDetals) == 0 {
			printer.Write(0, "StatefulSetPods:\t <none>\n")
		} else {
			printer.Write(0, "StatefulSetPods:\n")
			printer.Write(1, "owner\townerKind\tpodName\tnamespace\thasHostPath\tcpuReq\tcpuLimit\tmemReq\tmemLimit\n")

			for _, pods := range stsPodDetals {
				for _, pod := range pods {
					printer.Write(1, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
						pod.OwnerRef, pod.OwnerRefKind, pod.PodName, pod.Namespace, fmt.Sprintf("%v", pod.HostPath == true), pod.CpuRequest, pod.CpuLimit, pod.MemRequest, pod.MemLimit)
				}
			}
		}

		if len(isPods) == 0 {
			printer.Write(0, "IsolatedPods:\t<none>\n")
		} else {
			printer.Write(0, "IsolatedPods\n  podName\tnamespace\thasHostPath\n")
			for _, pod := range isPods {
				printer.Write(1, "%s\t%s\t%s\n", pod.PodName, pod.Namespace, fmt.Sprintf("%v", pod.HostPath == true))
			}
		}

		if len(nodeDetails) == 0 {
			printer.Write(0, "Node:\tnone\n")
		} else {
			printer.Write(0, "Node:\n")
			printer.Write(1, "nodeName\tmaxPods\tcurrentPods\tgpu\tschedule\tcpuAllocatable\tmemAllocatable\tcpuAllocated\tmemAllocated\n")
			for _, node := range nodeDetails {
				printer.Write(1, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					node.NodeName, node.MaxPods, node.CurrentPods, fmt.Sprintf("%v", node.GpuNode == true), fmt.Sprintf("%v", node.Schedule == true), node.CpuAllocatable, node.MemAllocatable, node.CpuAllocated, node.MemAllocated)
			}
		}

		if len(pdbDetails) == 0 {
			printer.Write(0, "PodDisruptionBudget:\tnone\n")
		} else {
			printer.Write(0, "PodDisruptionBudget:\n")
			for _, pdb := range pdbDetails {
				printer.Write(0, "pdbName:\t%s\n", pdb.PdbName)
				printer.Write(0, "pdbNamespace:\t%s\n", pdb.PdbNamespace)
				printer.Write(0, "pdbMinAvailable:\t%s\n", pdb.PdbMinAvailable)
				printer.Write(0, "pdbMaxUnavailable:\t%s\n", pdb.PdbMaxUnavailable)
				printer.Write(0, "pdbAllowed:\t%s\n", pdb.PdbAllowed)
				if len(pdb.PodDetails) != 0 {
					printer.Write(1, "owner\townerKind\tpodName\tnamespace\tnodeName\n")
					for _, pod := range pdb.PodDetails {
						printer.Write(1, "%s\t%s\t%s\t%s\t%s\n", pod.OwnerRef, pod.OwnerRefKind, pod.PodName, pod.Namespace, pod.NodeName)
					}
				}
			}

		}

		return nil
	})

}
