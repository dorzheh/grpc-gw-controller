// Author  <dorzheho@cisco.com>

package common

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"text/template"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/heroku/docker-registry-client/registry"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"cisco.com/son/apphcd/api/v1/appmanager"
	appcommon "cisco.com/son/apphcd/app/common"
)

// GetSubDirs returns a content of root directory
func GetSubDirs(rootPath string) ([]string, error) {
	var c []string
	subDirs, err := ioutil.ReadDir(rootPath)
	if err != nil {
		return nil, err
	}

	for _, d := range subDirs {
		c = append(c, d.Name())
	}

	return c, nil
}

// AppTypeToAppCycle converts application type provided by request to appropriate cycle that is
// used internally by controller
func AppTypeToAppCycle(appType string) (string, error) {
	logrus.Debug("Setting cycle for the application")

	var cycle string
	var err error

	switch appType {
	case TypePeriodic, TypeCronJob:
		cycle = TypePeriodic
	case TypeDaemon, TypeDeployment:
		cycle = TypeDaemon
	case TypeRunOnce, TypeJob:
		cycle = TypeRunOnce
	default:
		err = fmt.Errorf("unsupported Application type: %s", appType)
	}
	logrus.Debug("The type is ", appType)
	return cycle, err
}

// AppCycleToApptype converts application type provided by request to appropriate kubernetes object type
func AppCycleToAppType(appCycle string) (string, error) {
	logrus.Debug("Setting a type for the application")

	var appType string
	var err error

	switch appCycle {
	case TypePeriodic, TypeCronJob:
		appType = TypeCronJob
	case TypeDaemon, TypeDeployment:
		appType = TypeDeployment
	case TypeRunOnce, TypeJob:
		appType = TypeJob
	default:
		err = fmt.Errorf("unsupported Application type: %s", appType)
	}

	logrus.Debug("The type is ", appType)
	return appType, err
}

// GenerateResponse creates appropriate response
func GenerateResponse(statusCode appmanager.Status, msg string, m proto.Message) (*appmanager.Response, error) {
	var err error
	resp := new(appmanager.Response)

	if m != nil {
		// Create body
		resp.Body, err = ptypes.MarshalAny(m)
		if err != nil {
			msg = err.Error()
		}
	}

	// Set timestamp
	resp.Timestamp = ptypes.TimestampNow()
	// Set status code
	resp.Status = statusCode
	// Set message
	resp.Message = msg

	// If the protobuf message is nil we assume that an error occurred hence the error message is printed
	if statusCode == appmanager.Status_ERROR {
		logrus.Error(msg)
	}

	logrus.WithFields(logrus.Fields{"service": "AppManager", "type": "grpc", "status": statusCode}).Info("Sending response")
	logrus.Debugf("Response message: %q", resp.String())

	return resp, nil
}

// ValidateDockerImage validates the following:
// - Access to the Docker repository
// - Tag availability
func ValidateDockerImage(repo, tag string) error {

	username := "" // anonymous
	password := "" // anonymous

	if strings.Contains(repo, viper.GetString(appcommon.EnvApphcPrivateDockerRegistry)) {
		imageFields := strings.SplitN(repo, "/", 2)
		if len(imageFields) == 1 {
			return fmt.Errorf("unsupported  docker repository format [ %s ]", repo)
		}

		apphRegistryUrl := "http://" + imageFields[0]
		r, err := registry.New(apphRegistryUrl, username, password)
		if err != nil {
			return fmt.Errorf("cannot connect to the registry %s : %v", imageFields[0], err)
		}

		tags, err := r.Tags(imageFields[1])
		if err != nil {
			return fmt.Errorf("repo %s not found in the registry %s: %v", imageFields[1], imageFields[0], err)
		}

		for _, t := range tags {
			if t == tag {
				return nil
			}
		}

		return fmt.Errorf("image %s:%s not found ", repo, tag)
	}

	// Try Docker HUB
	dockerHubUrl := "https://registry-1.docker.io/"

	hub, err := registry.NewInsecure(dockerHubUrl, username, password)
	if err != nil {
		return fmt.Errorf("cannot connect to the registry %s : %v", dockerHubUrl, err)
	}

	var image string
	if strings.HasPrefix(repo, "docker.io") {
		image = strings.SplitN(repo, "/", 2)[1]
	} else {
		image = repo
	}

	tags, err := hub.Tags(image)
	if err != nil {
		return fmt.Errorf("repo %s not found in the registry %s: %v", image, dockerHubUrl, err)
	}

	for _, t := range tags {
		if tag == t {
			return nil
		}
	}

	return fmt.Errorf("image %s:%s not found ", repo, tag)
}

// Configuration file example:
// templates.url.monitor.apph.apps.daemon:  https://{{ .MonitorEndpoint }}/d/sondaemon/son-health-report-daemon-apps?var-app_base_name={{ .AppName }}
// templates.url.monitor.apph.apps.periodic: http://{{ .MonitorEndpoint }}/d/sonperiod/son-health-report-periodic-apps?var-app_base_name={{ .AppName }}
// templates.url.monitor.apph.apps.runonce: http://{{ .MonitorEndpoint }}/d/sonrunonc/son-health-report-runonce-apps?var-app_base_name={{ .AppName }}
// templates.url.monitor.apph.services: http://{{ .MonitorEndpoint }}/d/sonservic/son-apphoster-service-health?orgId=1
// templates.url.monitor.apph.nodes: http://{{ .MonitorEndpoint }}/d/apphnodes/son-apphoster-nodes?orgId=1&var-server={{ .Hostname }}:9100
func GetAppMonitorUrl(endpoint, appName, appCycle string) (string, error) {

	type App struct {
		MonitorEndpoint string
		AppName         string
	}

	var tmplt string
	switch appCycle {
	case TypeDaemon:
		tmplt = viper.GetString(TemplatesUrlMonitorAppsDaemon)

	case TypePeriodic:
		tmplt = viper.GetString(TemplatesUrlMonitorAppsPeriodic)

	case TypeRunOnce:
		tmplt = viper.GetString(TemplatesUrlMonitorAppsRunonce)
	}

	t, err := template.New("app").Parse(tmplt)
	if err != nil {
		return "", err
	}

	var url bytes.Buffer

	n := &App{MonitorEndpoint: endpoint, AppName: appName}
	if err := t.Execute(&url, n); err != nil {
		return "", err
	}

	return url.String(), nil
}

func Validate(requester CreateUpgradeUpdateRequester) error {
	if requester.GetCycle() == TypePeriodic {
		if requester.GetCyclePeriodicAttr().GetMinStartHour() > requester.GetCyclePeriodicAttr().GetMaxStartHour() {
			return fmt.Errorf("max_start_hour must be greater than min_start_hour")
		}
	}

	return nil
}

// PeriodicToCronString converts Periodic properties to Kubernetes property
func PeriodicToCronString(req CreateUpgradeUpdateRequester) string {
	var weekDays []string
	var cronString string

	c := req.GetCyclePeriodicAttr()

	if c.MinStartHour > c.MaxStartHour {
		var hours []string
		hours = append(hours, fmt.Sprintf("%d", c.MinStartHour))
		next := c.MinStartHour
		for {
			if next == c.MaxStartHour {
				break
			}
			if next == 23 {
				next = 0
			} else {
				next += 1
			}

			hours = append(hours, fmt.Sprintf("%d", next))
		}

		cronString = fmt.Sprintf("*/%d %s * * ", c.GetIntervalMin(), strings.Join(hours, ","))

	} else {

		cronString = fmt.Sprintf("*/%d %d-%d * * ", c.GetIntervalMin(), c.GetMinStartHour(), c.GetMaxStartHour())
	}

	d := c.GetWorkingDays()
	if d.GetSunday() {
		weekDays = append(weekDays, "0")
	}

	if d.GetMonday() {
		weekDays = append(weekDays, "1")
	}

	if d.GetTuesday() {
		weekDays = append(weekDays, "2")
	}

	if d.GetWednesday() {
		weekDays = append(weekDays, "3")
	}

	if d.GetThursday() {
		weekDays = append(weekDays, "4")
	}

	if d.GetFriday() {
		weekDays = append(weekDays, "5")
	}

	if d.GetSaturday() {
		weekDays = append(weekDays, "6")
	}

	cronString += strings.Join(weekDays, ",")

	return cronString
}

func CronStringToCyclePeriodicRespAttr(cron string) (*appmanager.CyclePeriodicRespAttr, error) {

	f := strings.Fields(cron)

	c := &appmanager.CyclePeriodicRespAttr{}
	imInt, err := strconv.Atoi(strings.Split(f[0], "/")[1])
	if err != nil {
		return nil, err
	}
	c.IntervalMin = uint32(imInt)

	if strings.Contains(f[1], "-") {
		rangeSlice := strings.Split(f[1], "-")
		minShInt, err := strconv.Atoi(rangeSlice[0])
		if err != nil {
			return nil, err
		}

		c.MinStartHour = uint32(minShInt)

		maxShInt, err := strconv.Atoi(rangeSlice[1])
		if err != nil {
			return nil, err
		}
		c.MaxStartHour = uint32(maxShInt)

	} else {

		hours := strings.Split(f[1], ",")
		minShInt, err := strconv.Atoi(hours[0])
		if err != nil {
			return nil, err
		}

		c.MinStartHour = uint32(minShInt)

		maxShInt , err :=  strconv.Atoi(hours[len(hours)-1])
		if err != nil {
			return nil, err
		}

		c.MaxStartHour = uint32(maxShInt)
	}

	for _, d := range strings.Split(f[4], ",") {
		switch d {
		case "0":
			c.WorkingDays = append(c.WorkingDays, WeekDaySunday)
			break

		case "1":
			c.WorkingDays = append(c.WorkingDays, WeekDayMonday)
			break

		case "2":
			c.WorkingDays = append(c.WorkingDays, WeekDayTuesday)
			break

		case "3":
			c.WorkingDays = append(c.WorkingDays, WeekDayWednesday)
			break

		case "4":
			c.WorkingDays = append(c.WorkingDays, WeekDayThursday)
			break

		case "5":
			c.WorkingDays = append(c.WorkingDays, WeekDayFriday)
			break

		case "6":
			c.WorkingDays = append(c.WorkingDays, WeekDaySaturday)
			break
		}
	}

	return c, nil
}
