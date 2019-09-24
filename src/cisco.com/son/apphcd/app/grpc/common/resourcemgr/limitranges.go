// Author <dorzheho@cisco.com>

package resourcemgr

import (
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CreateUpdateLimitRange creates or updates LimitRange object
func CreateUpdateLimitRange(c *kubernetes.Clientset, namespace string, cpu float64, memory uint32, defaultCpu , defaultMemory string) error {
	lr := &corev1.LimitRange{}
	lr.Name = namespace
	lr.Namespace = namespace
	lr.Spec = corev1.LimitRangeSpec{}

	// Iterate over the following types: PODs and Containers
	for _, t := range []corev1.LimitType{corev1.LimitTypePod, corev1.LimitTypeContainer} {
		item := corev1.LimitRangeItem{}
		item.Type = t
		item.Max = corev1.ResourceList{}
		item.Min = corev1.ResourceList{}
		item.DefaultRequest = corev1.ResourceList{}

		// In case CPU limits provided
		if cpu > 0 {
			// Set Minimal value for request
			item.Min[corev1.ResourceCPU] = resource.MustParse(defaultCpu)
			// Set Maximal value
			item.Max[corev1.ResourceCPU] = resource.MustParse(fmt.Sprintf("%.2f", cpu))
			// In case type is container set default value for request hence every created container will receive the default value
			if t == corev1.LimitTypeContainer {
				item.DefaultRequest[corev1.ResourceCPU] = resource.MustParse(defaultCpu)
			}
		}

		// In case Memory limits provided
		if memory > 0 {
			// Set minimal value
			item.Min[corev1.ResourceMemory] = resource.MustParse(defaultMemory)
			// Set Maximal value
			item.Max[corev1.ResourceMemory] = resource.MustParse(fmt.Sprintf("%dMi", memory))
			// In case type is container set default value for request hence every created container will receive the default
			if t == corev1.LimitTypeContainer {
				item.DefaultRequest[corev1.ResourceMemory] = resource.MustParse(defaultMemory)
			}
		}

		// Append item to the spec list
		lr.Spec.Limits = append(lr.Spec.Limits, item)
	}

	// Try to create an object in Kubernetes
	if _, err := c.CoreV1().LimitRanges(namespace).Create(lr); err != nil {
		// In case the limits already exists update it
		if errors.IsAlreadyExists(err) {
			if _, err := c.CoreV1().LimitRanges(namespace).Update(lr); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

// DeleteLimitRange deletes appropriate limits object
func DeleteLimitRange(c *kubernetes.Clientset, namespace string) error {
	err := c.CoreV1().LimitRanges(namespace).Delete(namespace, &metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}

	return err
}

// GetResourcesLimit fetches CPU and Memory limits from existing LimitRanges object
func GetResourcesLimit(c *kubernetes.Clientset, namespace string) (cpu float64, memory uint32, err error) {
	// Get existing limits
	el, err := c.CoreV1().LimitRanges(namespace).Get(namespace, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return cpu, memory, nil
	}

	// We need only first available item
	item := el.Spec.Limits[0]


	// In case CPU appears in the configuration
	if v, ok := item.Max[corev1.ResourceCPU]; ok {
		value := v.String()
		if strings.HasSuffix(value, "m") {
			// Remove "m" (millicores)
			var i int
			i, err = strconv.Atoi(value[0 : len(value)-1])
			if err != nil {
				return
			}
			// Convert to float64
			cpu = float64(i) / 1000

		} else {
			// Parse string value to float64
			cpu, err = strconv.ParseFloat(value, 64)
			if err != nil {
				return
			}
		}
	}

	// In case Memory appears in the configuration
	if v, ok := item.Max[corev1.ResourceMemory]; ok {
		// Get the value and remove the suffix "Mi"
		value := v.String()
		m := value[0 : len(value)-2]
		var memInt int
		memInt, err = strconv.Atoi(m)
		if err != nil {
			return
		}

		memory = uint32(memInt)
	}

	return
}
