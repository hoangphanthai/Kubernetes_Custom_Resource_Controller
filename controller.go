package main

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/hoangphanthai/Kubernetes_Custom_Resource_Controller/api/types/v1alpha1"

	clientV1alpha1 "github.com/hoangphanthai/Kubernetes_Custom_Resource_Controller/clientset/v1alpha1"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	coreV1Types "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/retry"
)

var applicationsClient clientV1alpha1.ApplicationInterface
var deploymentsClient v1.DeploymentInterface
var statefulsetsClient v1.StatefulSetInterface

var servicesClient coreV1Types.ServiceInterface
var secretsClient coreV1Types.SecretInterface
var configMapsClient coreV1Types.ConfigMapInterface

func watchResources() {

	applicationsClient = clientSet2.Applications(workingNspace)
	deploymentsClient = clientSet1.AppsV1().Deployments(workingNspace)
	statefulsetsClient = clientSet1.AppsV1().StatefulSets(workingNspace)
	servicesClient = clientSet1.CoreV1().Services(workingNspace)
	secretsClient = clientSet1.CoreV1().Secrets(workingNspace)
	configMapsClient = clientSet1.CoreV1().ConfigMaps(workingNspace)

	preList, conErr := applicationsClient.List(metav1.ListOptions{})

	if conErr != nil {
		showMessage(conErr.Error())
	} else {
		fmt.Printf("There currently %d application(s) in [%s] namespace \n", len(preList.Items), workingNspace)

		for {

			/* 	applicationsFromStore := store.List()
			fmt.Printf("application in store: %d\n", len(applicationsFromStore))
			*/

			var ad, de, co *v1alpha1.ApplicationList
			curList, err := applicationsClient.List(metav1.ListOptions{})
			if err != nil {
				// In case connection is lost. e.g. connecting/disconnecting VPN or network down
				showMessage(err.Error())
				curList = preList
			} else {

				// Getting newly added, deleted and unchanged applications list
				ad, de, co = appsListComparion(preList, curList)

				// Actions on newly DELETED Applications
				if de != nil && len(de.Items) != 0 {
					fmt.Println()
					showMessage("Found Applications deleted!")

					deletePolicy := metav1.DeletePropagationForeground

					for _, delApp := range de.Items {
						// 1.1 Deployment deleting
						fmt.Print(time.Unix(time.Now().Unix(), 0))
						fmt.Print(":  ")
						fmt.Printf("Process for deleted application %q \n", delApp.Name)

						err := deploymentsClient.Delete(context.TODO(), delApp.Name+"-deployment", metav1.DeleteOptions{
							PropagationPolicy: &deletePolicy})
						if err != nil {
							//fmt.Println(err.Error())
						} else {
							showMessage("---Deleting Deployment")
						}

						// 1.2 Delete external secret if exists
						err = secretsClient.Delete(context.TODO(), delApp.Name+"-externalsecret", metav1.DeleteOptions{})
						if err != nil {
							//fmt.Println(err.Error())
						} else {
							showMessage("---Deleting External Secret")
						}

						// 1.3 Disable Postgres cluster if it was enabled
						if delApp.Spec.Database.Enable {
							disableDBCluster(&delApp)
						}
					}
				}

				// Actions on newly ADDED Applications
				if ad != nil && len(ad.Items) != 0 {
					fmt.Println()
					showMessage("Found Applications added!")

					for _, addApp := range ad.Items {
						fmt.Print(time.Unix(time.Now().Unix(), 0))
						fmt.Print(":  ")
						fmt.Printf("Process for added application %q \n", addApp.Name)
						if addApp.Spec.Database.Enable {
							enableDBCluster(&addApp)
						}

						createDeployment(&addApp)

						// Update reload to false for first time creating if it is enabled
						if addApp.Spec.Externalsecret.Reload {
							addApp.Spec.Externalsecret.Reload = false
							_, _ = clientSet2.Applications(workingNspace).Update(&addApp, metav1.UpdateOptions{})
						}
					}
				}

				// Up to here after 2-second loop, there should be no change in applications list
				// This means curList.Names == preList.Names or preList.Names is nil (just created for 1st time)
				if co != nil && len(co.Items) != 0 {

					for _, curApp := range co.Items {

						// Step 3.1.: Checking Database setting

						// Get the previous (state) of curApp
						preApp := getAppFromName(curApp.Name, preList)

						if !curApp.Spec.Database.Enable { // 3.1.1 Database is currently Disabled
							if preApp.Spec.Database.Enable { // If previous Database state was enabled -> Disable Postgres cluster
								disableDBCluster(&preApp)
							}
						} else { // 3.1.2 Database is currently enabled

							if !preApp.Spec.Database.Enable {
								enableDBCluster(&curApp) // If previous Database state was disabled then re-Enable
							} else { // Database was and is currently Enabled

								// Check for DB spec updates so that later reconcile if possible
								dbSpecChanged := !reflect.DeepEqual(curApp.Spec.Database, preApp.Spec.Database)
								configMapChanged := !reflect.DeepEqual(curApp.Spec.Database.Spec.Configmap, preApp.Spec.Database.Spec.Configmap)
								secretChanged := !reflect.DeepEqual(curApp.Spec.Database.Spec.Secret, preApp.Spec.Database.Spec.Secret)

								// Check for the previous versions of Configmap and Secret, it exists then they are considered valid
								// (but maybe not containing key "POSTGRES_PASSWORD")

								_, errCfg := configMapsClient.Get(context.TODO(), curApp.Spec.Database.Spec.Configmap.Name, metav1.GetOptions{})
								configmapExists := (errCfg == nil)
								_, errSc := secretsClient.Get(context.TODO(), curApp.Spec.Database.Spec.Secret.Name, metav1.GetOptions{})
								secretExists := (errSc == nil)

								// Check whether there has an equivalent statefulset
								oneSflset, err := statefulsetsClient.Get(context.TODO(), curApp.Name+"-statefulset", metav1.GetOptions{})
								if err != nil {
									// If there is no statefulset with the name as curApp.Name (caused by deleting) -> re-create statefulset
									fmt.Println()
									showMessage(err.Error())
									enableDBCluster(&curApp)
								} else if dbSpecChanged {

									// Configmap
									if configMapChanged {
										showMessage(" -> Database's configmap changed")
										configMap := curApp.Spec.Database.Spec.Configmap
										if curApp.Spec.Database.Spec.Configmap.Name == preApp.Spec.Database.Spec.Configmap.Name {
											if configmapExists {
												// Update if existing configmap with the same name
												retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
													_, updateErr := configMapsClient.Update(context.TODO(), &configMap, metav1.UpdateOptions{})
													return updateErr
												})
												if retryErr != nil {
													showMessage(retryErr.Error())
												}
											} else {
												// Create new if no existing configmap with the same name
												_, err2 := configMapsClient.Create(context.TODO(), &configMap, metav1.CreateOptions{})
												configmapExists = err2 == nil
											}
										} else {
											// Configmaps' names are different, delete the old configmap if exists
											deletePolicy := metav1.DeletePropagationForeground
											_ = configMapsClient.Delete(context.TODO(), preApp.Spec.Database.Spec.Configmap.Name, metav1.DeleteOptions{
												PropagationPolicy: &deletePolicy})

											// Add the new configmap
											if configmapExists {
												// Update if existing configmap with the same name
												retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
													_, updateErr := configMapsClient.Update(context.TODO(), &configMap, metav1.UpdateOptions{})
													return updateErr
												})
												if retryErr != nil {
													showMessage(retryErr.Error())
												} else {
													configmapExists = true
												}
											} else {
												// Create new if no existing configmap with the same name
												_, err2 := configMapsClient.Create(context.TODO(), &configMap, metav1.CreateOptions{})
												configmapExists = err2 == nil
											}
										}
									}

									// Secret
									if secretChanged {
										showMessage(" -> Database's secret changed")
										oneSecret := curApp.Spec.Database.Spec.Secret
										if curApp.Spec.Database.Spec.Secret.Name == preApp.Spec.Database.Spec.Secret.Name {
											if secretExists {
												// Update if existing secret with the same name
												retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
													_, updateErr := secretsClient.Update(context.TODO(), &oneSecret, metav1.UpdateOptions{})
													return updateErr
												})
												if retryErr != nil {
													showMessage(retryErr.Error())
												} else {
													secretExists = true
												}
											} else {
												// Create new if no existing secret with the same name
												_, err2 := secretsClient.Create(context.TODO(), &oneSecret, metav1.CreateOptions{})
												secretExists = (err2 == nil)
											}
										} else {
											// Secrets' names are different, delete the old secret if exists
											deletePolicy := metav1.DeletePropagationForeground
											_ = secretsClient.Delete(context.TODO(), preApp.Spec.Database.Spec.Secret.Name, metav1.DeleteOptions{
												PropagationPolicy: &deletePolicy})

											// Add the new secret
											if curApp.Spec.Database.Spec.Secret.Name != curApp.Name+"-dbdefaultsecret" {
												// Check if the secret is not given by the user
												if secretExists {
													// Update if existing secret with the same name
													retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
														_, updateErr := secretsClient.Update(context.TODO(), &oneSecret, metav1.UpdateOptions{})
														return updateErr
													})
													if retryErr != nil {
														showMessage(retryErr.Error())
													} else {
														secretExists = true
													}
												} else {
													// Create new if not existing secret with the same name
													_, err2 := secretsClient.Create(context.TODO(), &oneSecret, metav1.CreateOptions{})
													secretExists = (err2 == nil)
												}
											} else {
												// Otherwise using the default database's secret
												secretExists = true
											}
										}
									}

									// 3.1.2. Update changes in Statefulset spec

									// Check for updates in Configmap and Secret
									if configMapChanged || secretChanged {

										if !configmapSecretIsValid(curApp.Spec.Database.Spec.Configmap.Name, curApp.Spec.Database.Spec.Secret.Name) {
											// The newly updated configmap or secret both do not contain the key "POSTGRES_PASSWORD" with non-empty value -> validate
											validateDBSecret(&curApp)
										}

										if configmapExists && secretExists {

											// Both database configmap and secret exist
											oneSflset.Spec.Template.Spec.Containers[0].EnvFrom = []apiv1.EnvFromSource{
												{ConfigMapRef: &apiv1.ConfigMapEnvSource{LocalObjectReference: apiv1.LocalObjectReference{Name: curApp.Spec.Database.Spec.Configmap.Name}}},
												{SecretRef: &apiv1.SecretEnvSource{LocalObjectReference: apiv1.LocalObjectReference{Name: curApp.Spec.Database.Spec.Secret.Name}}},
											}
										} else {

											// Either database configmap or secret exists
											if configmapExists {
												oneSflset.Spec.Template.Spec.Containers[0].EnvFrom = []apiv1.EnvFromSource{
													{ConfigMapRef: &apiv1.ConfigMapEnvSource{LocalObjectReference: apiv1.LocalObjectReference{Name: curApp.Spec.Database.Spec.Configmap.Name}}},
												}
											}
											if secretExists {
												oneSflset.Spec.Template.Spec.Containers[0].EnvFrom = []apiv1.EnvFromSource{
													{SecretRef: &apiv1.SecretEnvSource{LocalObjectReference: apiv1.LocalObjectReference{Name: curApp.Spec.Database.Spec.Secret.Name}}},
												}
											}
											if !configmapExists && !secretExists {
												// Both database configmap and secret not exist
												showMessage("Warning: Both database configmap and secret NOT exist")
												oneSflset.Spec.Template.Spec.Containers[0].EnvFrom = []apiv1.EnvFromSource{}
											}
										}
									}

									// Cluster size
									if curApp.Spec.Database.Spec.Clustersize != preApp.Spec.Database.Spec.Clustersize {
										oneSflset.Spec.Replicas = &(curApp.Spec.Database.Spec.Clustersize)
										fmt.Println()
										showMessage("Postgres cluster size changed")
									}

									// Disk size
									if curApp.Spec.Database.Spec.Disksize != preApp.Spec.Database.Spec.Disksize {
										showMessage("Es tut mir leid! Sie können die Festplattengröße nicht ändern!")
									}

									retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
										_, updateErr := statefulsetsClient.Update(context.TODO(), oneSflset, metav1.UpdateOptions{})
										return updateErr
									})

									if retryErr != nil {
										showMessage(retryErr.Error())
									} else {
										showMessage("---Statefulset updated successfully...")
									}

								} else {
									//fmt.Println("NO updates in the Database spec")
								}
							}
						}

						// Step 3.2 External's secret processing

						// extSecretIsValid is used to check if external secret is provided and valid
						var extSecretIsValid = false

						urlPre := preApp.Spec.Externalsecret.URL
						urlCur := curApp.Spec.Externalsecret.URL

						// Get the previous version of secret
						oneSecret, errExSecret := secretsClient.Get(context.TODO(), curApp.Name+"-externalsecret", metav1.GetOptions{})
						// Get the map[string][string] data from URL secret
						exSecretMap, err := getEvnMap(urlCur)

						if ((urlCur == urlPre) && curApp.Spec.Externalsecret.Reload) || (urlCur != urlPre) {
							// If user wants to reload the external secret or URL is changed by user
							if err != nil {
								if len(urlCur) > 0 {
									showMessage("The external secret (via URL) is changed or requested to reload, but invalid or inaccessible!")
								}
								// If previous external secret exists, it will be used
								extSecretIsValid = (errExSecret == nil)
							} else {
								// Now the external JSON content is valid, next is to check whether external secret exists, or not exists -> create, update
								if errExSecret != nil {
									// Previous external secret is not existing -> create new from given URL
									err3 := createExternalSecret(curApp.Name, exSecretMap)
									if err3 != nil {
										showMessage("An error occurs when creating external secret in K8s")
										showMessage(err3.Error())
										extSecretIsValid = false
									} else {
										extSecretIsValid = true
										showMessage(" -> New external secret is created successfully!")
									}
								} else {
									// External secret exists -> update it
									oneSecret.StringData = exSecretMap
									retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
										_, updateErr := secretsClient.Update(context.TODO(), oneSecret, metav1.UpdateOptions{})
										return updateErr
									})

									if retryErr != nil {
										showMessage(retryErr.Error())
										showMessage("---Previous version of external secret remains")
									} else {
										showMessage("---External secret updated successfully...")
									}

									// Either old secret or the updated one is valid
									extSecretIsValid = true
								}
							}
							// Setting the reload to false to avoid repetition
							if curApp.Spec.Externalsecret.Reload {
								curApp.Spec.Externalsecret.Reload = false
								_, _ = clientSet2.Applications(workingNspace).Update(&curApp, metav1.UpdateOptions{})
							}
						} else {
							// If user does not want to reload the external secret.
							if errExSecret == nil {
								// If previous external secret exists, it will be used
								extSecretIsValid = true
							} else {
								// If previous external secret not exists, but the external secret URL is given
								// Then there should be someone deleting the secret in the mean time, try to re-create (reconcile)
								if err != nil {
									// Invalid or inaccessible URL
									extSecretIsValid = false
								} else {
									err3 := createExternalSecret(curApp.Name, exSecretMap)
									if err3 != nil {
										showMessage("An error occurs when re-creating external secret in K8s")
										showMessage(err3.Error())
										extSecretIsValid = false
									} else {
										extSecretIsValid = true
										showMessage(" -> Previous external secret is re-created!")
									}
								}
							}
						}

						// Step 3.3.: Deployment comparing and processing

						// Check whether exists database configmap or secret on k8s
						_, err = configMapsClient.Get(context.TODO(), curApp.Spec.Database.Spec.Configmap.Name, metav1.GetOptions{})
						configmapExists := (err == nil)

						_, err = secretsClient.Get(context.TODO(), curApp.Spec.Database.Spec.Secret.Name, metav1.GetOptions{})
						secretExists := (err == nil)

						// Adding default values to curApp's specs if it is not set (or caused by deployment default values setting)
						listOfAppContainers := curApp.Spec.Template.Spec.Containers

						// Update the env to application to ensure consistency between app and deployment containers
						if curApp.Spec.Database.Enable {
							for i := 0; i < len(listOfAppContainers); i++ {
								if configmapExists && secretExists {
									// Both database configmap and secret exists
									listOfAppContainers[i].EnvFrom = []apiv1.EnvFromSource{
										{ConfigMapRef: &apiv1.ConfigMapEnvSource{LocalObjectReference: apiv1.LocalObjectReference{Name: curApp.Spec.Database.Spec.Configmap.Name}}},
										{SecretRef: &apiv1.SecretEnvSource{LocalObjectReference: apiv1.LocalObjectReference{Name: curApp.Spec.Database.Spec.Secret.Name}}},
									}
								} else {
									// Either database configmap or secret exists
									if configmapExists {
										listOfAppContainers[i].EnvFrom = []apiv1.EnvFromSource{
											{ConfigMapRef: &apiv1.ConfigMapEnvSource{LocalObjectReference: apiv1.LocalObjectReference{Name: curApp.Spec.Database.Spec.Configmap.Name}}},
										}
									}
									if secretExists {
										listOfAppContainers[i].EnvFrom = []apiv1.EnvFromSource{
											{SecretRef: &apiv1.SecretEnvSource{LocalObjectReference: apiv1.LocalObjectReference{Name: curApp.Spec.Database.Spec.Secret.Name}}},
										}
									}
								}
								if extSecretIsValid {
									listOfAppContainers[i].VolumeMounts = []apiv1.VolumeMount{{Name: "externalsecret", MountPath: "/external-secret"}}
								}

								// Update other default values
								if len(listOfAppContainers[i].ImagePullPolicy) == 0 {
									listOfAppContainers[i].ImagePullPolicy = apiv1.PullIfNotPresent
								}
								if len(listOfAppContainers[i].TerminationMessagePath) == 0 {
									listOfAppContainers[i].TerminationMessagePath = apiv1.TerminationMessagePathDefault
								}
								if len(listOfAppContainers[i].TerminationMessagePolicy) == 0 {
									listOfAppContainers[i].TerminationMessagePolicy = apiv1.TerminationMessageReadFile
								}
								if len(listOfAppContainers[i].Ports) != 0 {
									for j := 0; j < len(listOfAppContainers[i].Ports); j++ {
										if len(listOfAppContainers[i].Ports[j].Protocol) == 0 {
											listOfAppContainers[i].Ports[j].Protocol = apiv1.ProtocolTCP
										}
									}
								}
							}
						} else {
							for i := 0; i < len(listOfAppContainers); i++ {
								if extSecretIsValid {
									listOfAppContainers[i].VolumeMounts = []apiv1.VolumeMount{{Name: "externalsecret", MountPath: "/external-secret"}}
								}
								// Update other default values
								if len(listOfAppContainers[i].ImagePullPolicy) == 0 {
									listOfAppContainers[i].ImagePullPolicy = apiv1.PullIfNotPresent
								}
								if len(listOfAppContainers[i].TerminationMessagePath) == 0 {
									listOfAppContainers[i].TerminationMessagePath = apiv1.TerminationMessagePathDefault
								}
								if len(listOfAppContainers[i].TerminationMessagePolicy) == 0 {
									listOfAppContainers[i].TerminationMessagePolicy = apiv1.TerminationMessageReadFile
								}
								if len(listOfAppContainers[i].Ports) != 0 {
									for j := 0; j < len(listOfAppContainers[i].Ports); j++ {
										if len(listOfAppContainers[i].Ports[j].Protocol) == 0 {
											listOfAppContainers[i].Ports[j].Protocol = apiv1.ProtocolTCP
										}
									}
								}
							}
						}

						// Checking for changes in deployment

						oneDep, err := deploymentsClient.Get(context.TODO(), curApp.Name+"-deployment", metav1.GetOptions{})

						if err != nil {
							fmt.Println()
							showMessage(err.Error())
							// 3.3.1 NOT EXIST - If there is no depoyment with the name as oneapp (caused by deleting)
							createDeployment(&curApp)

							// For newly created deployment, the deployment status is not right away updated on application but after 2-second loop
							// Update reload value to False for first time creating
							if curApp.Spec.Externalsecret.Reload {
								curApp.Spec.Externalsecret.Reload = false
								_, _ = clientSet2.Applications(workingNspace).Update(&curApp, metav1.UpdateOptions{})
							}

						} else {

							replicasChanged := !(curApp.Spec.Replicas == int32(*oneDep.Spec.Replicas))
							containersChanged := !reflect.DeepEqual(curApp.Spec.Template.Spec.Containers, oneDep.Spec.Template.Spec.Containers)

							// There still a case here with config map and secret changes detection
							// That means it is not able to handle the change in content themself of config map and secret
							// Like the case of external secret when the name remains the same.
							// Possibly to enforce an deployment rollout once it detects a change in db's configmap/secret

							if replicasChanged || containersChanged {
								retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
									// Retrieve the latest version of deployment before attempting to update
									// RetryOnConflict uses exponential backoff to avoid exhausting the apiserver

									if replicasChanged {
										showMessage("Deployment replicas changed -> reconciling!")
										// -> Update replicas count
										oneDep.Spec.Replicas = &(curApp.Spec.Replicas)
									}

									if containersChanged {
										// Changes in specs -> Update containers' specs
										showMessage("Deployment template's spec changed -> reconciling!")
										oneDep.Spec.Template.Spec.Containers = curApp.Spec.Template.Spec.Containers
									}

									// Create volume for deployent pod when external secret is valid
									if extSecretIsValid {
										oneDep.Spec.Template.Spec.Volumes = []apiv1.Volume{
											{
												Name: "externalsecret",
												VolumeSource: apiv1.VolumeSource{
													Secret: &apiv1.SecretVolumeSource{
														SecretName: curApp.Name + "-externalsecret",
													},
												},
											}}
									}

									_, updateErr := deploymentsClient.Update(context.TODO(), oneDep, metav1.UpdateOptions{})
									return updateErr
								})

								if retryErr != nil {
									showMessage(retryErr.Error())

								} else {
									showMessage("--Deployment updated successfully...")
								}
							}

							// Update deployment status to the application for the kubectl describe command
							curApp.Spec.Deploymentstatus.Status = oneDep.Status
							_, _ = clientSet2.Applications(workingNspace).Update(&curApp, metav1.UpdateOptions{})
						}
					}
				}
			}
			preList = curList
			time.Sleep(2 * time.Second)
		}
	}
}

func getAppFromName(appName string, list *v1alpha1.ApplicationList) v1alpha1.Application {
	var temp v1alpha1.Application
	for _, s1 := range list.Items {
		if s1.Name == appName {
			return s1
		}
	}
	return temp
}

func appsListComparion(preApp, curApp *v1alpha1.ApplicationList) (addL, delL, comL *v1alpha1.ApplicationList) {
	// addL refers to the newly added apps list, delL is for newly deleted, and comL is for unchanged list
	ad := &v1alpha1.ApplicationList{}
	de := &v1alpha1.ApplicationList{}
	co := &v1alpha1.ApplicationList{}

	if len(preApp.Items) == 0 {
		if len(curApp.Items) == 0 {
			return nil, nil, nil
		}
		return curApp, nil, nil
	} else if len(curApp.Items) == 0 {
		return nil, preApp, nil
	} else {
		// Loop three times

		// 1st to find prev app not in curApp,
		for _, s1 := range curApp.Items {
			found := false
			for _, s2 := range preApp.Items {
				if s1.Name == s2.Name {
					found = true
					break
				}
			}
			// App not found. We add it to newly added Apps List
			if !found {
				ad.Items = append(ad.Items, s1)
			}
		}

		// 2nd loop to find cur app not in preApp
		for _, s1 := range preApp.Items {
			found := false
			for _, s2 := range curApp.Items {
				if s1.Name == s2.Name {
					found = true
					break
				}
			}
			// App not found. We add it to newly deleted Apps List
			if !found {
				de.Items = append(de.Items, s1)
			}
		}

		// 3rd loop to find the common apps list between the curApp and preApp
		for _, s1 := range curApp.Items {
			for _, s2 := range preApp.Items {
				if s1.Name == s2.Name {
					co.Items = append(co.Items, s1)
				}
			}
		}

		return ad, de, co
	}
}

func showMessage(s string) {
	fmt.Print(time.Unix(time.Now().Unix(), 0))
	fmt.Print(":  ")
	fmt.Println(s)
}

// WatchResources is
/* func WatchResources(clientSet clientV1alpha1.ExampleV1Alpha1Interface) cache.Store {
	applicationStore, applicationController := cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(lo metav1.ListOptions) (result runtime.Object, err error) {
				return clientSet.Applications("default").List(lo)
			},
			WatchFunc: func(lo metav1.ListOptions) (watch.Interface, error) {
				return clientSet.Applications("default").Watch(lo)
			},
		},
		&v1alpha1.Application{},
		1*time.Minute,
		cache.ResourceEventHandlerFuncs{},
	)

	go applicationController.Run(wait.NeverStop)
	return applicationStore
} */
