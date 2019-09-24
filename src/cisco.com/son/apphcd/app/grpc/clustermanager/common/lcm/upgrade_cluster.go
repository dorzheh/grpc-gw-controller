// Author <dorzheho@cisco.com>

package lcm

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	appcommon "cisco.com/son/apphcd/app/common"
	"cisco.com/son/apphcd/app/common/mutex"
	clumgrcommon "cisco.com/son/apphcd/app/grpc/clustermanager/common"
)

const (
	cmdPrepare                       = "apph-upgrade.sh -o prepare"
	cmdUpgrade                       = "apph-upgrade.sh -o upgrade"
	cmdClean                         = "apph-upgrade.sh -o clean"
	componentNameApphosterController = "controller"
)

type Component struct {
	Name         string `json:"Name"`
	Status       string `json:"Status"`
	ErrorMessage string `json:"ErrorMessage"`
}

type Upgrade struct {
	LastUpgrade  *timestamp.Timestamp `json:"LastUpgrade"`
	Status       string               `json:"Status"`
	ErrorMessage string               `json:"ErrorMessage"`
	Components   []*Component         `json:"Components"`
}

// UpgradeCluster responses for upgrading Apphoster cluster
func UpgradeCluster(sshclient *ssh.Client, kc *kubernetes.Clientset, ns *v1.Namespace) {

	u := &Upgrade{}
	u.Status = "OK"

	// Create session
	s, err := sshclient.NewSession()
	if err != nil {
		finalize(sshclient, kc, ns, u, err)
		return
	}

	defer s.Close()

	remoteHost := sshclient.RemoteAddr()
	cmd := fmt.Sprintf("%s -d %s", cmdPrepare, viper.GetString(appcommon.EnvApphcPrivateDockerRegistry))
	logrus.WithFields(logrus.Fields{"host": remoteHost, "cmd": cmd}).Info("Preparing for upgrade")
	o, err := s.Output(cmd)
	fields := strings.Fields(string(o))
	if err != nil {
		finalize(sshclient, kc, ns, u, err)
		return
	}

	logrus.WithFields(logrus.Fields{"host": remoteHost, "cmd": cmd, "components": fields}).Info("Preparing for upgrade")

	gitEndpoint := "http://intucell:intucell@" + viper.GetString(appcommon.EnvApphcGitServerEndpoint)

	// Wait for the group of subroutines
	var wg sync.WaitGroup

	upgradeController := false
	for _, component := range fields {
		if component == componentNameApphosterController {
			upgradeController = true
		} else {
			wg.Add(1)
			go func(u *Upgrade, component string) {
				defer wg.Done()
				c := new(Component)
				c.Name = component

				defer func(u *Upgrade, c *Component) {
					u.Components = append(u.Components, c)
				}(u, c)

				s, err := sshclient.NewSession()
				if err != nil {
					c.Status = "ERROR"
					c.ErrorMessage = err.Error()
					logrus.Error(err)
					return
				}

				defer s.Close()

				c.Status = "OK"

				cmd := fmt.Sprintf("%s -d %s -c %s -g %s",
					cmdUpgrade, viper.GetString(appcommon.EnvApphcPrivateDockerRegistry), component, gitEndpoint)

				logrus.WithFields(logrus.Fields{"host": remoteHost, "component": component, "cmd": cmd}).Info("Upgrading component")
				o, err := s.CombinedOutput(cmd)
				if logrus.GetLevel() == logrus.DebugLevel {
					logrus.WithFields(logrus.Fields{"host": remoteHost, "component": component, "output": string(o)}).Debug("Upgrading component")
				}

				if err != nil {
					c.Status = "ERROR"
					c.ErrorMessage = err.Error()
					logrus.Error(err)
					return
				}

				logrus.WithFields(logrus.Fields{"host": remoteHost, "component": component, "cmd": cmd, "status": "OK"}).Info("Upgrading component")

			}(u, component)

			time.Sleep(time.Second * 1)
		}
	}

	wg.Wait()

	if ! upgradeController {
		finalize(sshclient, kc, ns, u, nil)
		return
	}

	c := new(Component)
	c.Name = componentNameApphosterController
	defer func(u *Upgrade, c *Component, err error) {
		if err == nil {
			c.Status = "OK"
		} else {
			c.Status = "ERROR"
			c.ErrorMessage = err.Error()
		}
		u.Components = append(u.Components, c)
		finalize(sshclient, kc, ns, u, err)
	}(u, c, err)

	s, err = sshclient.NewSession()
	if err != nil {
		return
	}

	defer s.Close()

	cmd = fmt.Sprintf("%s -d %s -c %s -g %s",
		cmdUpgrade, viper.GetString(appcommon.EnvApphcPrivateDockerRegistry), componentNameApphosterController, gitEndpoint)
	logrus.WithFields(logrus.Fields{"host": remoteHost, "component": componentNameApphosterController, "cmd": cmd}).Info("Upgrading component")
	o, err = s.CombinedOutput(cmd)

	// The below lines will be executed only if controller is running outside of AppHoster (for example , running in IDE)
	if logrus.GetLevel() == logrus.DebugLevel {
		logrus.WithFields(logrus.Fields{"host": remoteHost, "component": componentNameApphosterController, "output": string(o)}).Info("Upgrading component")
	}

	if err != nil {
		return
	}

	logrus.WithFields(logrus.Fields{"host": remoteHost, "component": componentNameApphosterController, "cmd": cmd, "status": "OK"}).Info("Upgrading component")
	logrus.Info("Upgrade completed successfully")
}

// finalize is the last function executed during upgrade:
// - cleans up temporary stuff
// - creates appropriate entries in the default namespace annotations
// - removes mutex
func finalize(sshclient *ssh.Client, kc *kubernetes.Clientset, ns *v1.Namespace, u *Upgrade, err error) {
	defer sshclient.Close()

	s, _ := sshclient.NewSession()

	remoteHost := sshclient.RemoteAddr()

	logrus.WithFields(logrus.Fields{"host": remoteHost, "cmd": cmdClean}).Info("Cleaning up")
	if err := s.Run(cmdClean); err != nil {
		return
	}

	logrus.WithFields(logrus.Fields{"host": remoteHost, "status": "OK"}).Info("Cleaning up")

	u.LastUpgrade = ptypes.TimestampNow()

	if err != nil {
		u.Status = "ERROR"
		u.ErrorMessage = err.Error()
		logrus.Error(err)
	} else {
		for _, s := range u.Components {
			if s.ErrorMessage != "" {
				u.Status = "ERROR"
				u.ErrorMessage = "AppHoster upgrade failed"
				break
			}
		}
	}

	b, _ := json.Marshal(u)

	appcommon.MapAdd(ns.Annotations, clumgrcommon.NamespaceAnnotationApphosterUpgradeStatus, string(b))
	_, _ = kc.CoreV1().Namespaces().Update(ns)
	mutex.Unlock(mutex.LockActionUpgradeCluster)
}
