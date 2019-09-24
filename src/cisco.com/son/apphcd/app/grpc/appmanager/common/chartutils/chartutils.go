// Author  <dorzheho@cisco.com>

package chartutils

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/otiai10/copy"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"

	"cisco.com/son/apphcd/api/v1/appmanager"
	appcommon "cisco.com/son/apphcd/app/common"
	appmgrcommon "cisco.com/son/apphcd/app/grpc/appmanager/common"
)

// CreateChart creates a new helm chart for appropriate application instance
func CreateChart(srcChart, trgtChart, chartType, appName string, data *appmgrcommon.AppInstanceData) error {
	logrus.WithFields(logrus.Fields{"instance": data.InstanceName,
		"version": data.RequestedVersion, "chart": trgtChart}).Info("Creating helm chart")

	// Copy template for appropriate chart type
	if err := copy.Copy(srcChart, trgtChart); err != nil {
		return err
	}

	// Create Chart.yaml file
	if err := createChartYaml(trgtChart, chartType, data); err != nil {
		return fmt.Errorf("cannot create Chart.yaml for application %s version %s",
			data.InstanceName, data.Annotations.Get(appmgrcommon.AppInstanceAnnotationVersion))
	}

	// Create values.yaml file
	if err := createValuesYaml(trgtChart, chartType, data); err != nil {
		return fmt.Errorf("cannot create Values.yaml for application %s version %s",
			data.InstanceName, data.Annotations.Get(appmgrcommon.AppInstanceAnnotationVersion))
	}

	// Create configmap file
	if len(data.AppConfigs) > 0 {
		if err := createConfigMap(trgtChart, data); err != nil {
			return err
		}
	}

	if len(data.Secrets) > 0 {
		if err := createSecrets(trgtChart, data); err != nil {
			return err
		}
	}

	logrus.WithFields(logrus.Fields{"instance": data.InstanceName,
		"version": data.RequestedVersion, "chart": trgtChart, "status": "OK"}).Info("Creating helm chart")

	return nil
}

func CopyChart(srcChart, trgtChart, instanceName, version string) error {
	logrus.WithFields(logrus.Fields{"instance": instanceName,
		"version": version, "src": srcChart, "target": trgtChart}).Info("Copying the chart")

	if err := copy.Copy(srcChart, trgtChart); err != nil {
		return err
	}

	logrus.WithFields(logrus.Fields{"instance": instanceName,
		"version": version, "src": srcChart, "target": trgtChart, "status": "OK"}).Info("Copying the chart")

	return nil
}

// DeleteChart deletes appropriate application instance chart
func DeleteChart(chartPath, instanceName, version string) error {
	logrus.WithFields(logrus.Fields{"instance": instanceName,
		"version": version, "chart": chartPath}).Info("Deleting helm chart")

	if err := os.RemoveAll(chartPath); err != nil {
		return err
	}

	logrus.WithFields(logrus.Fields{"instance": instanceName,
		"version": version, "chart": chartPath, "status": "OK"}).Info("Deleting helm chart")

	return nil
}

// createValuesYaml creates Values.yaml file
func createValuesYaml(chartPath, chartType string, data *appmgrcommon.AppInstanceData) error {
	logrus.Debug("Constructing values.yaml data")

	var buffer bytes.Buffer

	buffer.WriteString(fmt.Sprintf("namespace: %s\n", data.TargetNamespace))
	buffer.WriteString("pullPolicy: IfNotPresent\n")
	buffer.WriteString("replicaCount: 1\n")
	buffer.WriteString("nodeSelector: {}\n")
	buffer.WriteString("affinity: {}\n")
	buffer.WriteString("liveness:\n")
	buffer.WriteString("  initialDelaySeconds: 60\n")
	buffer.WriteString("  periodSeconds: 10\n")
	buffer.WriteString("  enabled: false\n")
	buffer.WriteString("readiness:\n")
	buffer.WriteString("  initialDelaySeconds: 60\n")
	buffer.WriteString("  periodSeconds: 10\n")
	buffer.WriteString("  enabled: false\n")
	buffer.WriteString("ingress:\n")
	buffer.WriteString("  enabled: false\n")

	switch chartType {
	case appmgrcommon.TypePeriodic:
		buffer.WriteString(fmt.Sprintf("schedule: '%s'\n", data.CyclePeriodicSched))
		buffer.WriteString("concurrencyPolicy: Forbid\n")
		buffer.WriteString("restartPolicy: OnFailure\n")
		buffer.WriteString("failedJobsHistoryLimit: 1\n")
		buffer.WriteString("successfulJobsHistoryLimit: 3\n")

	case appmgrcommon.TypeRunOnce:
		buffer.WriteString("restartPolicy: OnFailure\n")
	}

	if !data.Labels.Empty() {
		buffer.WriteString("labels:\n")
		for k, v := range data.Labels {
			buffer.WriteString(fmt.Sprintf(" %s: '%s'\n", k, v))
		}
	}

	if !data.Annotations.Empty() {
		buffer.WriteString("annotations:\n")
		for k, v := range data.Annotations {
			buffer.WriteString(fmt.Sprintf(" %s: '%s'\n", k, v))
		}
	}

	buffer.WriteString("image: \n")
	buffer.WriteString("  repository: " + data.Image.Repository)
	buffer.WriteString(fmt.Sprintf("\n  tag: '%s'", data.Image.Tag))

	buffer.WriteString("\nenv:\n")
	for k, v := range data.EnvVars {
		buffer.WriteString(fmt.Sprintf(" %s: '%s'\n", k, v))
	}

	buffer.WriteString("service:\n")
	buffer.WriteString("  type: ClusterIP\n")
	buffer.WriteString("  name: " + data.InstanceName)
	if len(data.Ports) > 0 {
		buffer.WriteString("\nports:")
		for _, port := range data.Ports {
			if port != nil {
				buffer.WriteString("\n  - name: " + port.Name)
				buffer.WriteString(fmt.Sprintf("\n    internalPort: '%d'", port.Number))
				buffer.WriteString("\n    protocol: " + port.Proto)
			}
		}
	}

	buffer.WriteString("\npersistence:\n")
	buffer.WriteString("  instance:\n")
	if data.InstanceStorageSize > 0 {
		buffer.WriteString("    enabled: true\n")
		buffer.WriteString(fmt.Sprintf("    size: %dGi\n", data.InstanceStorageSize))
	} else {
		buffer.WriteString("   enabled: false\n")
	}

	buffer.WriteString("  shared:\n")
	if data.SharedStorageEnabled {
		buffer.WriteString("    enabled: true\n")
	} else {
		buffer.WriteString("    enabled: false\n")
	}

	buffer.WriteString("configs:\n")
	if len(data.AppConfigs) > 0 {
		buffer.WriteString("  enabled: true\n")
	} else {
		buffer.WriteString("  enabled: false\n")
	}

	buffer.WriteString("secrets:\n")
	if len(data.Secrets) > 0 {
		buffer.WriteString("  enabled: true\n")
	} else {
		buffer.WriteString("  enabled: false\n")
	}

	buffer.WriteString("resources: {}\n")

	f, err := os.OpenFile(filepath.Join(chartPath, "values.yaml"), os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	if err != nil {
		return err
	}

	if _, err := f.Write(buffer.Bytes()); err != nil {
		return err
	}

	logrus.Debug("The file values.yaml is created")

	return nil
}

// createChartYaml creates Chart.yaml file for appropriate chart
func createChartYaml(chartPath, chartType string, data *appmgrcommon.AppInstanceData) error {
	logrus.Debugf("Creating chart %s metadata", data.InstanceName)

	if data.Description == "" {
		data.Description = fmt.Sprintf("Application %s , version %s",
			data.InstanceName, data.Annotations.Get(appmgrcommon.AppInstanceAnnotationVersion))
	}
	var msg string

	switch chartType {
	case appmgrcommon.TypePeriodic:
		msg = fmt.Sprintf("name: %s\nversion: %s\ndescription: %s\nengine: gotpl\nkeywords:\n- periodic\n- cron",
			appcommon.MapGet(data.Annotations, appmgrcommon.AppInstanceAnnotationTemplateName),
			data.Annotations.Get(appmgrcommon.AppInstanceAnnotationVersion), data.Description)
	case appmgrcommon.TypeDaemon:
		msg = fmt.Sprintf("apiVersion: v1\nname: %s\nversion: %s\ndescription: %s\nengine: gotpl\nkeywords:\n- daemon\n- deployment",
			appcommon.MapGet(data.Annotations, appmgrcommon.AppInstanceAnnotationTemplateName),
			data.Annotations.Get(appmgrcommon.AppInstanceAnnotationVersion), data.Description)
	case appmgrcommon.TypeRunOnce:
		msg = fmt.Sprintf("name: %s\nversion: %s\ndescription: %s\nengine: gotpl\nkeywords:\n- run_once\n- job",
			appcommon.MapGet(data.Annotations, appmgrcommon.AppInstanceAnnotationTemplateName),
			data.Annotations.Get(appmgrcommon.AppInstanceAnnotationVersion), data.Description)
	}

	f := []byte(msg)
	return ioutil.WriteFile(filepath.Join(chartPath, "Chart.yaml"), f, 0644)
}

// createConfigMap creates configuration file that will be used as a key for appropriate configmap
func createConfigMap(chartPath string, data *appmgrcommon.AppInstanceData) error {
	logrus.Debug("Creating configmap")

	configPath := filepath.Join(chartPath, "resources/configs")
	if err := os.MkdirAll(configPath, 0755); err != nil {
		return err
	}

	for k, v := range data.AppConfigs {
		if err := ioutil.WriteFile(filepath.Join(configPath, k), []byte(v), 0644); err != nil {
			return err
		}
	}

	return nil
}

// createSecrets creates secret files
func createSecrets(chartPath string, data *appmgrcommon.AppInstanceData) error {
	logrus.Debug("Creating secrets")

	secretPath := filepath.Join(chartPath, "resources/secrets")
	if err := os.MkdirAll(secretPath, 0755); err != nil {
		return err
	}

	for k, v := range data.Secrets {
		if err := ioutil.WriteFile(filepath.Join(secretPath, k), []byte(v), 0644); err != nil {
			return err
		}
	}

	return nil
}

// ParseLastGoodConfig parses appropriate values.yaml file
func ParseLastGoodConfig(valuesFile string) (map[string]interface{}, error) {

	// Allocation a map to hold the old values
	m := make(map[string]interface{})

	// Check whether the file exists
	if _, err := os.Stat(valuesFile); err != nil {
		return nil, err
	}

	// Read the file content
	f, err := ioutil.ReadFile(valuesFile)
	if err != nil {
		return nil, err
	}

	// Parse the content to the map
	if err := yaml.Unmarshal(f, m); err != nil {
		return nil, err
	}

	return m, nil
}

// SetChartData appends a new data that will be used for creating metadata for appropriate application instance.
// In case "reuseValues" is true , Controller will try to create a new metadata that will be based on existing one.
func SetChartData(data *appmgrcommon.AppInstanceData, req appmgrcommon.CreateUpgradeUpdateRequester, chartDir string,
	reusedValues map[string]interface{}) error {
	// Shouldn't happen but lets check it
	if data == nil {
		return errors.New("FATAL: Application data is nil")
	}

	// Initialize "AppConfigs" map
	if data.AppConfigs == nil {
		data.AppConfigs = appcommon.MakeMap()
	}

	// Initialize "Secrets" map
	if data.Secrets == nil {
		data.Secrets = appcommon.MakeMap()
	}

	// Initialize "Image" structure
	if data.Image == nil {
		data.Image = &appmgrcommon.Image{}
	}

	// Initialize "EnvVars" map
	if data.EnvVars == nil {
		data.EnvVars = appcommon.MakeMap()
	}

	// Append default environment variables
	data.EnvVars.Add(appmgrcommon.EnvVarAppInstanceSecretsDir, appmgrcommon.AppInstanceDefaultSecretsDir)
	data.EnvVars.Add(appmgrcommon.EnvVarAppInstanceConfigDir, appmgrcommon.AppInstanceDefaultConfigDir)
	data.EnvVars.Add(appmgrcommon.EnvVarAppInstanceStorageDir, appmgrcommon.AppInstanceDefaultStorageDir)
	data.EnvVars.Add(appmgrcommon.EnvVarAppStorageDir, appmgrcommon.AppDefaultStorageDir)

	// In case "reuseValues" flag is true
	if len(reusedValues) > 0 {
		// Check whether "configs" is enabled or not
		if reusedValues["configs"].(map[interface{}]interface{})["enabled"] == true {
			path := filepath.Join(chartDir, "resources/configs")
			files, err := ioutil.ReadDir(path)
			if err != nil {
				return err
			}

			// Create configuration files
			for _, f := range files {
				c, err := ioutil.ReadFile(filepath.Join(path, f.Name()))
				if err != nil {
					return err
				}

				// Add configs to the AppConfigs map
				data.AppConfigs.Add(f.Name(), string(c))
			}
		}

		// Check whether "secrets" enabled or not
		if reusedValues["secrets"].(map[interface{}]interface{})["enabled"].(bool) == true {
			path := filepath.Join(chartDir, "resources/secrets")

			files, err := ioutil.ReadDir(path)
			if err != nil {
				return err
			}

			// Create secrets
			for _, f := range files {
				s, err := ioutil.ReadFile(filepath.Join(path, f.Name()))
				if err != nil {
					return err
				}

				// Add secrets to the Secrets map
				data.Secrets.Add(f.Name(), string(s))
			}
		}

		// Docker image repository
		data.Image.Repository = reusedValues["image"].(map[interface{}]interface{})["repository"].(string)

		// Docker image name
		data.Image.Name = reusedValues["image"].(map[interface{}]interface{})["name"].(string)

		// Docker image tag
		data.Image.Tag = reusedValues["image"].(map[interface{}]interface{})["tag"].(string)

		// In case the old configuration contains appropriate annotations
		if e, ok := reusedValues["annotations"]; ok {
			for k, v := range e.(map[interface{}]interface{}) {
				if !strings.Contains(k.(string), "create.helm-controller") {
					// If annotation exists,  override it with the new value
					// only in case the existing annotation doesn't have value
					if !data.Annotations.KeyHasValue(k.(string)) {
						data.Annotations.Add(k.(string), v.(string))
					}
				}

				// Set RequestedVersion to the version obtained from annotation
				if k == appmgrcommon.AppInstanceAnnotationVersion {
					data.RequestedVersion = v.(string)
				}
			}
		}

		// Set state of the instance
		state := reusedValues["annotations"].(map[interface{}]interface{})[appmgrcommon.AppInstanceAnnotationState].(string)
		s, err := strconv.Atoi(state)
		if err != nil {
			return err
		}

		data.State = appmanager.AppStateAfterDeployment(s)

		// Check if the old configuration contains labels information
		if e, ok := reusedValues["labels"]; ok {
			for k, v := range e.(map[interface{}]interface{}) {
				// If label exists,  override it with the new value
				// only in case the existing label doesn't have value
				if !data.Labels.KeyHasValue(k.(string)) {
					data.Labels.Add(k.(string), v.(string))
				}
			}
		}

		// If environment variables exist
		if e, ok := reusedValues["env"]; ok {
			for k, v := range e.(map[interface{}]interface{}) {
				// Add variable to the "EnvVars" map
				data.EnvVars.Add(k.(string), v.(string))
			}
		}

		// If persistent storage for instance is enabled
		p := reusedValues["persistence"].(map[interface{}]interface{})
		if p["instance"].(map[interface{}]interface{})["enabled"].(bool) == true {
			// Get the size of the storage
			str := p["instance"].(map[interface{}]interface{})["size"].(string)
			// No storage by default
			data.InstanceStorageSize = 0
			sizeStr := str[0 : len(str)-2]
			if sizeStr != "" {
				// Convert the size from string to integer
				sizeDigit, err := strconv.Atoi(sizeStr)
				if err != nil {
					return err
				}

				// Set the new data with the storage size of a running instance
				data.InstanceStorageSize = sizeDigit
			}
		}

		// If shared persistent storage enabled
		if p["shared"].(map[interface{}]interface{})["enabled"].(bool) == true {
			data.SharedStorageEnabled = true
		}

		// If ports persist in the map
		if ports, ok := reusedValues["ports"].([]interface{}); ok {
			// Iterate over the ports
			for _, p := range ports {
				port := &appmgrcommon.Port{}
				for k, v := range p.(map[interface{}]interface{}) {
					switch k {
					case "name":
						port.Name = v.(string)
					case "internalPort":
						port.Number = uint32(v.(int))
					case "protocol":
						port.Proto = v.(string)
					}
				}

				data.Ports = append(data.Ports, port)
			}
		}

		if sched, ok := reusedValues["schedule"].(string); ok {
			data.CyclePeriodicSched = sched
		}
	}

	// Set filters for annotations
	excludeAnnotationsFromRequest := []string{appmgrcommon.AppAnnotationBaseName, appmgrcommon.AppAnnotationCycle,
		appmgrcommon.AppAnnotationSchedule, appmgrcommon.AppInstanceAnnotationRootGroupId,
		appmgrcommon.AppInstanceAnnotationGroupId, appmgrcommon.AppInstanceAnnotationId, appmgrcommon.AppInstanceAnnotationVersion,
		appmgrcommon.AppInstanceAnnotationTemplateName, appmgrcommon.AppInstanceAnnotationPersistentVolumeSize}

	// Add all annotation that came with request except those that were filtered
	appcommon.MapMerge(data.Annotations, req.GetAnnotations(), excludeAnnotationsFromRequest...)

	// Set the version in annotation. Use the version that came with request
	appcommon.MapAdd(data.Annotations, appmgrcommon.AppInstanceAnnotationVersion, data.RequestedVersion)

	// Set filters for labels
	excludeLabelsFromRequest := []string{appmgrcommon.AppAnnotationBaseName, appmgrcommon.AppAnnotationCycle,
		appmgrcommon.AppInstanceAnnotationRootGroupId, appmgrcommon.AppInstanceAnnotationGroupId,
		appmgrcommon.AppInstanceAnnotationId, appmgrcommon.AppInstanceAnnotationVersion,
		appmgrcommon.AppInstanceAnnotationTemplateName, appmgrcommon.AppInstanceAnnotationPersistentVolumeSize}

	// Add all labels that came with request except those that were filtered
	appcommon.MapMerge(data.Labels, req.GetLabels(), excludeLabelsFromRequest...)

	// Set labels. Use labels that came with request
	appcommon.MapMerge(data.Labels, req.GetLabels())

	// Add appropriate information related to the "draft" development tool
	if v := appcommon.MapGet(req.GetLabels(), appmgrcommon.DraftAppLabelBasename); v != "" {
		appcommon.MapAdd(data.Labels, "draft", data.InstanceName)
	}

	// Add appropriate information related to the "monitoring" system (Prometheus)
	appcommon.MapAdd(data.Labels, appmgrcommon.MonAppLabelBasename, appcommon.MapGet(data.Annotations, appmgrcommon.AppAnnotationBaseName))

	// Merge configs
	appcommon.MapMerge(data.AppConfigs, req.GetAppConfigs())

	// Merge environment variables
	appcommon.MapMerge(data.EnvVars, req.GetEnvVars())

	// Merge secrets
	appcommon.MapMerge(data.Secrets, req.GetSecrets())

	// If the application type is periodic and the periodic attributes arne't empty
	if appcommon.MapGet(data.Annotations, appmgrcommon.AppAnnotationCycle) == appmgrcommon.TypePeriodic {
		if req.GetCyclePeriodicAttr() != nil {
			// Set the new schedule time
			data.CyclePeriodicSched = appmgrcommon.PeriodicToCronString(req)
		}

		appcommon.MapAdd(data.Annotations, appmgrcommon.AppAnnotationSchedule, data.CyclePeriodicSched)
	}

	if req.GetDescription() != "" {
		data.Description = req.GetDescription()
	}

	if req.GetSpec().GetImage().GetRepo() != "" {
		data.Image.Repository = req.GetSpec().GetImage().GetRepo()
		appcommon.MapAdd(data.Annotations, appmgrcommon.AppInstanceAnnotationImageRepoName, data.Image.Repository)
	}

	if req.GetSpec().GetImage().GetTag() != "" {
		data.Image.Tag = req.GetSpec().GetImage().GetTag()
		appcommon.MapAdd(data.Annotations, appmgrcommon.AppInstanceAnnotationImageTag, data.Image.Tag)
	}

	// Get ports information
	if len(req.GetSpec().GetPorts()) > 0 {
		// Always override with the new values
		data.Ports = []*appmgrcommon.Port{}
		for _, port := range req.GetSpec().GetPorts() {
			p := &appmgrcommon.Port{}
			pn := port.GetName()
			if pn == "" {
				p.Name = strings.Replace(strings.ToLower(req.GetName()), "_", "-", -1)
			} else {
				p.Name = strings.Replace(strings.ToLower(pn), "_", "-", -1)
			}

			p.Proto = port.GetProto().String()
			p.Number = port.GetNumber()
			data.Ports = append(data.Ports, p)
		}
	}

	// Get storage size from request
	// If the field is empty , the value will be 0
	newSize := int(req.GetSpec().GetResources().GetPersistentStorage())

	// In case the application is not a new application and existing size not equal to the new size
	if data.InstanceStorageSize != -1 && data.InstanceStorageSize != newSize {
		// We need to recreate the instance
		data.NextAction = appmgrcommon.AppInstanceDataNextActionRecreate
		data.DeleteInstanceStorage = true
	}

	// If we need to reuse the old values
	if len(reusedValues) > 0 {
		data.DeleteInstanceStorage = false
		// And the size came with request is not 0
		if newSize > 0 {
			if data.InstanceStorageSize != newSize {
				data.DeleteInstanceStorage = true
			}
			// Only then set the new storage size - that came in request
			data.InstanceStorageSize = int(newSize)
		}
	} else {
		// Otherwise set to the new size
		data.InstanceStorageSize = newSize
	}

	if req.GetSharedStorage() > 0 {
		data.SharedStorageEnabled = true
	}

	// Add the storage size to annotations
	data.Annotations.Add(appmgrcommon.AppInstanceAnnotationPersistentVolumeSize, strconv.Itoa(data.InstanceStorageSize))

	// Add environment variable for Root Group ID if exists
	rootGroupId := data.Annotations.Get(appmgrcommon.AppInstanceAnnotationRootGroupId)
	if rootGroupId != "" {
		data.EnvVars.Add(appmgrcommon.EnvVarAppInstanceRootGroupId, rootGroupId)
	}

	// Add environment variable for Application name
	data.EnvVars.Add(appmgrcommon.EnvVarAppName, appcommon.MapGet(data.Annotations, appmgrcommon.AppAnnotationBaseName))

	// Add environment variable for Application instance name
	data.EnvVars.Add(appmgrcommon.EnvVarAppInstanceName, data.InstanceName)

	// Add environment variable for Application instance ID
	data.EnvVars.Add(appmgrcommon.EnvVarAppInstanceId, appcommon.MapGet(data.Annotations, appmgrcommon.AppInstanceAnnotationId))

	// Add environment variable for Group ID
	data.EnvVars.Add(appmgrcommon.EnvVarAppInstanceGroupId, data.Annotations.Get(appmgrcommon.AppInstanceAnnotationGroupId))

	// Add environment variable for the application instance version
	data.EnvVars.Add(appmgrcommon.EnvVarAppInstanceVersion, data.RequestedVersion)

	// Add environment variable for the FlexAPI Host IP/FQDN
	data.EnvVars.Add(appmgrcommon.EnvVarAppInstanceFlexApiHost, viper.GetString(appcommon.EnvApphcAppFlexApiHost))

	// Add environment variable for the FlexAPI Host IP/FQDN
	data.EnvVars.Add(appmgrcommon.EnvVarAppInstanceFlexApiPort, viper.GetString(appcommon.EnvApphcAppFlexApiPort))

	// Add environment variable for Flex Apps AppId
	data.EnvVars.Add(appmgrcommon.EnvVarAppInstanceAppId, appcommon.MapGet(data.Secrets, appmgrcommon.AttributeAppId))

	// Add environment variable for Flex Apps SecretKey
	data.EnvVars.Add(appmgrcommon.EnvVarAppInstanceSecretKey, appcommon.MapGet(data.Secrets, appmgrcommon.AttributeSecretKey))

	return nil
}
