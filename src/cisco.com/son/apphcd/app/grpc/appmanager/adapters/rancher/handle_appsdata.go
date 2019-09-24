// Author <dorzheho@cisco.com>

package rancher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"cisco.com/son/apphcd/api/v1/appmanager"
	"cisco.com/son/apphcd/app/grpc/appmanager/adapters/rancher/apiclient"
	"cisco.com/son/apphcd/app/grpc/appmanager/common"
	"cisco.com/son/apphcd/app/grpc/appmanager/common/chartutils"
	"cisco.com/son/apphcd/app/grpc/common/rancher"
)

// createUpgradeApps responsible for making a decision whether the application
// instance should be created, upgraded/updated
func createUpgradeApps(apiClient *rancher.MasterClient, appInstances []*common.AppInstanceData, catalogId string) ([]*appmanager.AppInstance, error) {

	// Wait for the group of subroutines
	var wg sync.WaitGroup
	var errors []error
	var doneList []*appmanager.AppInstance

	_, cancel := context.WithCancel(context.Background())
	defer cancel() // Make sure it's called to release resources even if no errors

	// iterate over the list of temporary data
	for _, instance := range appInstances {
		if instance.NextAction != common.AppInstanceDataNextActionNone {
			// create a separate subroutine for a particular action
			wg.Add(1)
			go func(instance *common.AppInstanceData) {
				defer wg.Done()

				switch instance.NextAction {
				case common.AppInstanceDataNextActionCreate:
					// Create a new application instance
					if err := apiclient.CreateAppInstance(apiClient, instance, instance.RequestedVersion); err != nil {
						errors = append(errors, err)
					} else {
						doneList = append(doneList, generateProtoData(instance, catalogId))
					}

				case common.AppInstanceDataNextActionUpgrade:
					// Upgrade application instance
					if err := apiclient.UpgradeAppInstance(apiClient, instance); err != nil {
						errors = append(errors, err)
					} else {
						doneList = append(doneList, generateProtoData(instance, catalogId))
					}

				case common.AppInstanceDataNextActionRecreate:
					if err := apiclient.DeleteAppInstance(apiClient, instance, instance.CurrentVersion); err != nil {
						errors = append(errors, err)
					} else {
						if err := apiclient.CreateAppInstance(apiClient, instance, instance.RequestedVersion); err != nil {
							err = fmt.Errorf("cannot recreate the application instance %s: %s", instance.InstanceName, err.Error())
							errors = append(errors, err)
							cancel()
						} else {
							doneList = append(doneList, generateProtoData(instance, catalogId))

						}
					}
				}
			}(instance)

			time.Sleep(time.Second * 1)
		}
	}

	wg.Wait()

	if len(errors) > 0 {
		return nil, errors[0]
	}

	return doneList, nil
}

// doApps responsible for making a decision whether the application
// instance should be created, upgraded or deleted
func deleteApps(apiClient *rancher.MasterClient, appInstances []*common.AppInstanceData, catalogId, appName,
	repoPath string, purge bool) ([]*appmanager.AppInstance, error) {

	// Wait for the group of subroutines
	var wg sync.WaitGroup
	var errors []error
	var doneList []*appmanager.AppInstance

	// iterate over the list of temporary data
	for _, instance := range appInstances {
		if instance.NextAction != common.AppInstanceDataNextActionNone {
			// create a separate subroutine for a particular action
			wg.Add(1)
			go func(instance *common.AppInstanceData) {
				defer wg.Done()

				switch instance.NextAction {
				case common.AppInstanceDataNextActionDelete:
					instance.DeleteInstanceStorage = true
					if err := apiclient.DeleteAppInstance(apiClient, instance, instance.CurrentVersion); err != nil {
						errors = append(errors, err)
					} else {
						// If need to remove metadata
						if purge {
							// Set path to the application root directory
							root := filepath.Join(repoPath, appName)
							// Set path to the chart
							chartPath := filepath.Join(root, instance.InstanceName)
							if instance.RequestedVersion != "" {
								chartPath = filepath.Join(chartPath, instance.RequestedVersion)
							}
							// Delete the chart
							if err := chartutils.DeleteChart(chartPath, instance.InstanceName, instance.RequestedVersion); err != nil {
								errors = append(errors, err)
							}
							// Add to the list
							doneList = append(doneList, generateProtoData(instance, catalogId))
							if len(doneList) == len(appInstances) {
								_ = os.RemoveAll(root)
							}
						} else {
							doneList = append(doneList, generateProtoData(instance, ""))
						}
					}
				}
			}(instance)

			time.Sleep(time.Second * 1)
		}
	}

	wg.Wait()

	if len(errors) > 0 {
		return nil, errors[0]
	}

	return doneList, nil
}
