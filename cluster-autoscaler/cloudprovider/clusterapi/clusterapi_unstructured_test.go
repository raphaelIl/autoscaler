/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package clusterapi

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
	"testing"
	"time"
)

func TestSetSize(t *testing.T) {
	initialReplicas := 1
	updatedReplicas := 5

	test := func(t *testing.T, testConfig *testConfig) {
		controller, stop := mustCreateTestController(t, testConfig)
		defer stop()

		testResource := testConfig.machineSet
		if testConfig.machineDeployment != nil {
			testResource = testConfig.machineDeployment
		}

		sr, err := newUnstructuredScalableResource(controller, testResource)
		if err != nil {
			t.Fatal(err)
		}

		gvr, err := sr.GroupVersionResource()
		if err != nil {
			t.Fatal(err)
		}

		err = sr.SetSize(updatedReplicas)
		if err != nil {
			t.Fatal(err)
		}

		s, err := sr.controller.managementScaleClient.Scales(testResource.GetNamespace()).
			Get(context.TODO(), gvr.GroupResource(), testResource.GetName(), metav1.GetOptions{})

		if s.Spec.Replicas != int32(updatedReplicas) {
			t.Errorf("expected %v, got: %v", updatedReplicas, s.Spec.Replicas)
		}

		replicas, found, err := unstructured.NestedInt64(sr.unstructured.Object, "spec", "replicas")
		if err != nil {
			t.Fatal(err)
		}
		if !found {
			t.Fatal("replicas = 0")
		}
		if replicas != int64(updatedReplicas) {
			t.Errorf("expected %v, got: %v", updatedReplicas, replicas)
		}
	}

	t.Run("MachineSet", func(t *testing.T) {
		test(t, createMachineSetTestConfig(
			RandomString(6),
			RandomString(6),
			RandomString(6),
			initialReplicas, map[string]string{
				nodeGroupMinSizeAnnotationKey: "1",
				nodeGroupMaxSizeAnnotationKey: "10",
			},
		))
	})

	t.Run("MachineDeployment", func(t *testing.T) {
		test(t, createMachineDeploymentTestConfig(
			RandomString(6),
			RandomString(6),
			RandomString(6),
			initialReplicas, map[string]string{
				nodeGroupMinSizeAnnotationKey: "1",
				nodeGroupMaxSizeAnnotationKey: "10",
			},
		))
	})
}

func TestReplicas(t *testing.T) {
	initialReplicas := 1
	updatedReplicas := 5

	test := func(t *testing.T, testConfig *testConfig) {
		controller, stop := mustCreateTestController(t, testConfig)
		defer stop()

		testResource := testConfig.machineSet
		if testConfig.machineDeployment != nil {
			testResource = testConfig.machineDeployment
		}

		sr, err := newUnstructuredScalableResource(controller, testResource)
		if err != nil {
			t.Fatal(err)
		}

		gvr, err := sr.GroupVersionResource()
		if err != nil {
			t.Fatal(err)
		}

		i, err := sr.Replicas()
		if err != nil {
			t.Fatal(err)
		}

		if i != initialReplicas {
			t.Errorf("expected %v, got: %v", initialReplicas, i)
		}

		// fetch and update machineSet
		s, err := sr.controller.managementScaleClient.Scales(testResource.GetNamespace()).
			Get(context.TODO(), gvr.GroupResource(), testResource.GetName(), metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}

		s.Spec.Replicas = int32(updatedReplicas)

		ch := make(chan error)
		checkDone := func(obj interface{}) (bool, error) {
			u, ok := obj.(*unstructured.Unstructured)
			if !ok {
				return false, nil
			}
			sr, err := newUnstructuredScalableResource(controller, u)
			if err != nil {
				return true, err
			}
			i, err := sr.Replicas()
			if err != nil {
				return true, err
			}
			if i != updatedReplicas {
				return true, fmt.Errorf("expected %v, got: %v", updatedReplicas, i)
			}
			return true, nil
		}
		handler := cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				match, err := checkDone(obj)
				if match {
					ch <- err
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				match, err := checkDone(newObj)
				if match {
					ch <- err
				}
			},
		}

		controller.machineSetInformer.Informer().AddEventHandler(handler)
		controller.machineDeploymentInformer.Informer().AddEventHandler(handler)

		_, err = sr.controller.managementScaleClient.Scales(testResource.GetNamespace()).
			Update(context.TODO(), gvr.GroupResource(), s, metav1.UpdateOptions{})
		if err != nil {
			t.Fatal(err)
		}

		lastErr := fmt.Errorf("no updates received yet")
		for lastErr != nil {
			select {
			case err = <-ch:
				lastErr = err
			case <-time.After(1 * time.Second):
				t.Fatal(fmt.Errorf("timeout while waiting for update. Last error was: %v", lastErr))
			}
		}
	}

	t.Run("MachineSet", func(t *testing.T) {
		test(t, createMachineSetTestConfig(RandomString(6), RandomString(6), RandomString(6), initialReplicas, nil))
	})

	t.Run("MachineDeployment", func(t *testing.T) {
		test(t, createMachineDeploymentTestConfig(RandomString(6), RandomString(6), RandomString(6), initialReplicas, nil))
	})
}

func TestSetSizeAndReplicas(t *testing.T) {
	initialReplicas := 1
	updatedReplicas := 5

	test := func(t *testing.T, testConfig *testConfig) {
		controller, stop := mustCreateTestController(t, testConfig)
		defer stop()

		testResource := testConfig.machineSet
		if testConfig.machineDeployment != nil {
			testResource = testConfig.machineDeployment
		}

		sr, err := newUnstructuredScalableResource(controller, testResource)
		if err != nil {
			t.Fatal(err)
		}

		i, err := sr.Replicas()
		if err != nil {
			t.Fatal(err)
		}

		if i != initialReplicas {
			t.Errorf("expected %v, got: %v", initialReplicas, i)
		}

		err = sr.SetSize(updatedReplicas)
		if err != nil {
			t.Fatal(err)
		}

		i, err = sr.Replicas()
		if err != nil {
			t.Fatal(err)
		}

		if i != updatedReplicas {
			t.Errorf("expected %v, got: %v", updatedReplicas, i)
		}
	}

	t.Run("MachineSet", func(t *testing.T) {
		test(t, createMachineSetTestConfig(
			RandomString(6),
			RandomString(6),
			RandomString(6),
			initialReplicas, map[string]string{
				nodeGroupMinSizeAnnotationKey: "1",
				nodeGroupMaxSizeAnnotationKey: "10",
			},
		))
	})

	t.Run("MachineDeployment", func(t *testing.T) {
		test(t, createMachineDeploymentTestConfig(
			RandomString(6),
			RandomString(6),
			RandomString(6),
			initialReplicas, map[string]string{
				nodeGroupMinSizeAnnotationKey: "1",
				nodeGroupMaxSizeAnnotationKey: "10",
			},
		))
	})
}
