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

// CreateUpdateResourceQuotas creates or updates ResourceQuota object
func CreateUpdateResourceQuotas(c *kubernetes.Clientset, namespace string, cpu float64, memory uint32) error {
	q := &corev1.ResourceQuota{}
	q.Name = namespace
	q.Namespace = namespace
	q.Spec = corev1.ResourceQuotaSpec{}
	q.Spec.Hard = corev1.ResourceList{}

	// In case CPU limits provided
	if cpu > 0 {
		q.Spec.Hard[corev1.ResourceCPU] = resource.MustParse(fmt.Sprintf("%.2f", cpu))
	}

	// In case Memory limits provided
	if memory > 0 {
		q.Spec.Hard[corev1.ResourceMemory] = resource.MustParse(fmt.Sprintf("%dMi", memory))
	}

	// Try to create an object in Kubernetes
	if _, err := c.CoreV1().ResourceQuotas(namespace).Create(q); err != nil {
		// In case the limits already exists update it
		if errors.IsAlreadyExists(err) {
			if _, err := c.CoreV1().ResourceQuotas(namespace).Update(q); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

// DeleteResourceQuotas deletes appropriate ResourceQuotas object
func DeleteResourceQuotas(c *kubernetes.Clientset, namespace string) error {
	return c.CoreV1().ResourceQuotas(namespace).Delete(namespace, &metav1.DeleteOptions{})
}

// GetResourceQuotas fetches CPU and Memory quotas from existing ResourceQuotas object
func GetResourceQuotas(c *kubernetes.Clientset, namespace string) (cpu float64, memory uint32, err error) {
	// Get existing quota
	q, err := c.CoreV1().ResourceQuotas(namespace).Get(namespace, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return cpu, memory, nil
	}

	// In case CPU appears in the configuration
	v := q.Spec.Hard.Cpu()
	if v.IsZero() {
		return
	}
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

	v = q.Spec.Hard.Memory()
	if v.IsZero() {
		return
	}

	value = v.String()
	m := value[0 : len(value)-2]
	var memInt int
	memInt, err = strconv.Atoi(m)
	if err != nil {
		return
	}

	memory = uint32(memInt)

	return
}
