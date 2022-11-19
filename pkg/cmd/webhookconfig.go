package main

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strings"

	rlv1beta1 "github.com/chenliu1993/resourcelimiter/api/v1beta1"
	rlv1beta2 "github.com/chenliu1993/resourcelimiter/api/v1beta2"
	"github.com/munnerz/goautoneg"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8sjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

var (
	webhookConfigName   = "resourcelimiter-checker"
	WebhookMutatePath   = "/mutate"
	WebhookValidatePath = "/validate"
	WebhookConvertPath  = "/convert"
)

func createOrUpdateWebhookConfiguration(clientset *kubernetes.Clientset, caPEM *bytes.Buffer, webhookService, webhookNamespace string, mutate bool) error {

	webhookConfigV1Client := clientset.AdmissionregistrationV1()

	infoLogger.Printf("Creating or updating the webhookconfiguration: %s", webhookConfigName)
	fail := admissionregistrationv1.Fail
	sideEffect := admissionregistrationv1.SideEffectClassNone
	var (
		validatingWebhookConfig *admissionregistrationv1.ValidatingWebhookConfiguration
		mutatingWebhookConfig   *admissionregistrationv1.MutatingWebhookConfiguration
	)
	if mutate {
		webhookConfigName = fmt.Sprintf("%s-mutate", webhookConfigName)
		mutatingWebhookConfig = &admissionregistrationv1.MutatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name:      webhookConfigName,
				Namespace: webhookNamespace,
			},
			Webhooks: []admissionregistrationv1.MutatingWebhook{{
				Name:                    "resourcelimiter.mutate.cliufreever.io",
				AdmissionReviewVersions: []string{"v1", "v1beta1"},
				SideEffects:             &sideEffect,
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					CABundle: caPEM.Bytes(), // self-generated CA for the webhook
					Service: &admissionregistrationv1.ServiceReference{
						Name:      webhookService,
						Namespace: webhookNamespace,
						Path:      &WebhookMutatePath,
					},
				},
				Rules: []admissionregistrationv1.RuleWithOperations{
					{
						Operations: []admissionregistrationv1.OperationType{
							admissionregistrationv1.Update,
							admissionregistrationv1.Create,
						},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{"resources.resourcelimiter.io"},
							APIVersions: []string{"v1beta1", "v1beta2"},
							Resources:   []string{"resourcelimiters"},
						},
					},
				},
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"resourcelimiter-mutate": "enabled",
					},
				},
				FailurePolicy: &fail,
			}},
		}
		foundWebhookConfig, err := webhookConfigV1Client.MutatingWebhookConfigurations().Get(context.TODO(), webhookConfigName, metav1.GetOptions{})
		if err != nil && apierrors.IsNotFound(err) {
			if _, err := webhookConfigV1Client.MutatingWebhookConfigurations().Create(context.TODO(), mutatingWebhookConfig, metav1.CreateOptions{}); err != nil {
				warningLogger.Printf("Failed to create the mutatingwebhookconfiguration: %s", webhookConfigName)
				return err
			}
			infoLogger.Printf("Created mutatingwebhookconfiguration: %s", webhookConfigName)
		} else if err != nil {
			warningLogger.Printf("Failed to check the mutatingwebhookconfiguration: %s", webhookConfigName)
			return err
		} else {
			// there is an existing mutatingWebhookConfiguration
			if len(foundWebhookConfig.Webhooks) != len(mutatingWebhookConfig.Webhooks) ||
				!(foundWebhookConfig.Webhooks[0].Name == mutatingWebhookConfig.Webhooks[0].Name &&
					reflect.DeepEqual(foundWebhookConfig.Webhooks[0].AdmissionReviewVersions, mutatingWebhookConfig.Webhooks[0].AdmissionReviewVersions) &&
					reflect.DeepEqual(foundWebhookConfig.Webhooks[0].SideEffects, mutatingWebhookConfig.Webhooks[0].SideEffects) &&
					reflect.DeepEqual(foundWebhookConfig.Webhooks[0].FailurePolicy, mutatingWebhookConfig.Webhooks[0].FailurePolicy) &&
					reflect.DeepEqual(foundWebhookConfig.Webhooks[0].Rules, mutatingWebhookConfig.Webhooks[0].Rules) &&
					reflect.DeepEqual(foundWebhookConfig.Webhooks[0].NamespaceSelector, mutatingWebhookConfig.Webhooks[0].NamespaceSelector) &&
					reflect.DeepEqual(foundWebhookConfig.Webhooks[0].ClientConfig.CABundle, mutatingWebhookConfig.Webhooks[0].ClientConfig.CABundle) &&
					reflect.DeepEqual(foundWebhookConfig.Webhooks[0].ClientConfig.Service, mutatingWebhookConfig.Webhooks[0].ClientConfig.Service)) {
				mutatingWebhookConfig.ObjectMeta.ResourceVersion = foundWebhookConfig.ObjectMeta.ResourceVersion
				if _, err := webhookConfigV1Client.MutatingWebhookConfigurations().Update(context.TODO(), mutatingWebhookConfig, metav1.UpdateOptions{}); err != nil {
					warningLogger.Printf("Failed to update the mutatingwebhookconfiguration: %s", webhookConfigName)
					return err
				}
				infoLogger.Printf("Updated the mutatingwebhookconfiguration: %s", webhookConfigName)
			}
			infoLogger.Printf("The mutatingwebhookconfiguration: %s already exists and has no change", webhookConfigName)
		}
	} else {
		webhookConfigName = fmt.Sprintf("%s-validate", webhookConfigName)
		validatingWebhookConfig = &admissionregistrationv1.ValidatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name:      webhookConfigName,
				Namespace: webhookNamespace,
			},
			Webhooks: []admissionregistrationv1.ValidatingWebhook{{
				Name:                    "resourcelimiter.validate.cliufreever.io",
				AdmissionReviewVersions: []string{"v1", "v1beta1"},
				SideEffects:             &sideEffect,
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					CABundle: caPEM.Bytes(), // self-generated CA for the webhook
					Service: &admissionregistrationv1.ServiceReference{
						Name:      webhookService,
						Namespace: webhookNamespace,
						Path:      &WebhookValidatePath,
					},
				},
				Rules: []admissionregistrationv1.RuleWithOperations{
					{
						Operations: []admissionregistrationv1.OperationType{
							admissionregistrationv1.Create,
							admissionregistrationv1.Update,
						},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{"resources.resourcelimiter.io"},
							APIVersions: []string{"v1beta1", "v1beta2"},
							Resources:   []string{"resourcelimiters"},
						},
					},
					{
						Operations: []admissionregistrationv1.OperationType{
							admissionregistrationv1.Create,
							admissionregistrationv1.Update,
						},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
					},
					{
						Operations: []admissionregistrationv1.OperationType{
							admissionregistrationv1.Create,
							admissionregistrationv1.Update,
						},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{"apps", "extensions"},
							APIVersions: []string{"v1"},
							Resources:   []string{"deployments", "daemonsets"},
						},
					},
				},
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"resourcelimiter-validate": "enabled",
					},
				},
				FailurePolicy: &fail,
			}},
		}
		foundWebhookConfig, err := webhookConfigV1Client.ValidatingWebhookConfigurations().Get(context.TODO(), webhookConfigName, metav1.GetOptions{})
		if err != nil && apierrors.IsNotFound(err) {
			if _, err := webhookConfigV1Client.ValidatingWebhookConfigurations().Create(context.TODO(), validatingWebhookConfig, metav1.CreateOptions{}); err != nil {
				warningLogger.Printf("Failed to create the validatingwebhookconfiguration: %s", webhookConfigName)
				return err
			}
			infoLogger.Printf("Created validatingwebhookconfiguration: %s", webhookConfigName)
		} else if err != nil {
			warningLogger.Printf("Failed to check the validatingwebhookconfiguration: %s", webhookConfigName)
			return err
		} else {
			// there is an existing validatingWebhookConfiguration
			if len(foundWebhookConfig.Webhooks) != len(validatingWebhookConfig.Webhooks) ||
				!(foundWebhookConfig.Webhooks[0].Name == mutatingWebhookConfig.Webhooks[0].Name &&
					reflect.DeepEqual(foundWebhookConfig.Webhooks[0].AdmissionReviewVersions, validatingWebhookConfig.Webhooks[0].AdmissionReviewVersions) &&
					reflect.DeepEqual(foundWebhookConfig.Webhooks[0].SideEffects, validatingWebhookConfig.Webhooks[0].SideEffects) &&
					reflect.DeepEqual(foundWebhookConfig.Webhooks[0].FailurePolicy, validatingWebhookConfig.Webhooks[0].FailurePolicy) &&
					reflect.DeepEqual(foundWebhookConfig.Webhooks[0].Rules, validatingWebhookConfig.Webhooks[0].Rules) &&
					reflect.DeepEqual(foundWebhookConfig.Webhooks[0].NamespaceSelector, validatingWebhookConfig.Webhooks[0].NamespaceSelector) &&
					reflect.DeepEqual(foundWebhookConfig.Webhooks[0].ClientConfig.CABundle, validatingWebhookConfig.Webhooks[0].ClientConfig.CABundle) &&
					reflect.DeepEqual(foundWebhookConfig.Webhooks[0].ClientConfig.Service, validatingWebhookConfig.Webhooks[0].ClientConfig.Service)) {
				validatingWebhookConfig.ObjectMeta.ResourceVersion = foundWebhookConfig.ObjectMeta.ResourceVersion
				if _, err := webhookConfigV1Client.ValidatingWebhookConfigurations().Update(context.TODO(), validatingWebhookConfig, metav1.UpdateOptions{}); err != nil {
					warningLogger.Printf("Failed to update the validatingwebhookconfiguration: %s", webhookConfigName)
					return err
				}
				infoLogger.Printf("Updated the validatingwebhookconfiguration: %s", webhookConfigName)
			}
			infoLogger.Printf("The validatingwebhookconfiguration: %s already exists and has no change", webhookConfigName)
		}
	}

	return nil
}

// // convertFunc is the user defined function for any conversion. The code in this file is a
// // template that can be use for any CR conversion given this function.
// type convertFunc func(Object *unstructured.Unstructured, version string) (*unstructured.Unstructured, metav1.Status)

// conversionResponseFailureWithMessagef is a helper function to create an AdmissionResponse
// with a formatted embedded error message.
func conversionResponseFailureWithMessagef(msg string, params ...interface{}) *v1beta1.ConversionResponse {
	return &v1beta1.ConversionResponse{
		Result: metav1.Status{
			Message: fmt.Sprintf(msg, params...),
			Status:  metav1.StatusFailure,
		},
	}

}

func statusErrorWithMessage(msg string, params ...interface{}) metav1.Status {
	return metav1.Status{
		Message: fmt.Sprintf(msg, params...),
		Status:  metav1.StatusFailure,
	}
}

func statusSucceed() metav1.Status {
	return metav1.Status{
		Status: metav1.StatusSuccess,
	}
}

var scheme = runtime.NewScheme()
var serializers = map[mediaType]runtime.Serializer{
	{"application", "json"}: k8sjson.NewSerializer(k8sjson.DefaultMetaFactory, scheme, scheme, false),
	{"application", "yaml"}: k8sjson.NewYAMLSerializer(k8sjson.DefaultMetaFactory, scheme, scheme),
}

type mediaType struct {
	Type, SubType string
}

func getInputSerializer(contentType string) runtime.Serializer {
	parts := strings.SplitN(contentType, "/", 2)
	if len(parts) != 2 {
		return nil
	}
	return serializers[mediaType{parts[0], parts[1]}]
}

func getOutputSerializer(accept string) runtime.Serializer {
	if len(accept) == 0 {
		return serializers[mediaType{"application", "json"}]
	}

	clauses := goautoneg.ParseAccept(accept)
	for _, clause := range clauses {
		for k, v := range serializers {
			switch {
			case clause.Type == k.Type && clause.SubType == k.SubType,
				clause.Type == k.Type && clause.SubType == "*",
				clause.Type == "*" && clause.SubType == "*":
				return v
			}
		}
	}

	return nil
}

// doConversion converts the requested object given the conversion function and returns a conversion response.
// failures will be reported as Reason in the conversion response.
func doConversion(convertRequest *v1beta1.ConversionRequest) *v1beta1.ConversionResponse {
	var convertedObjects []runtime.RawExtension
	switch convertRequest.DesiredAPIVersion {
	case "resources.resourcelimiter.io/v1beta2":
		for _, obj := range convertRequest.Objects {
			unstructuredCR := &unstructured.Unstructured{}
			if err := unstructuredCR.UnmarshalJSON(obj.Raw); err != nil {
				klog.Error(err)
				return conversionResponseFailureWithMessagef("failed to unmarshall object (%v) with error: %v", string(obj.Raw), err)
			}

			cr := &rlv1beta1.ResourceLimiter{
				Spec: rlv1beta1.ResourceLimiterSpec{
					Targets: unstructuredCR.Object["targets"].([]rlv1beta1.ResourceLimiterNamespace),
					Types:   unstructuredCR.Object["types"].(map[rlv1beta1.ResourceLimiterType]string),
					Applied: unstructuredCR.Object["applied"].(bool),
				},
			}

			newVer, status := convertV1beta1IntoV1beta2(cr)
			if status.Status != metav1.StatusSuccess {
				klog.Error(status.String())
				return &v1beta1.ConversionResponse{
					Result: status,
				}
			}
			convertedObjects = append(convertedObjects, runtime.RawExtension{Object: newVer})

		}
	case "resources.resourcelimiter.io/v1beta1":
		for _, obj := range convertRequest.Objects {
			unstructuredCR := &unstructured.Unstructured{}
			if err := unstructuredCR.UnmarshalJSON(obj.Raw); err != nil {
				klog.Error(err)
				return conversionResponseFailureWithMessagef("failed to unmarshall object (%v) with error: %v", string(obj.Raw), err)
			}

			cr := &rlv1beta2.ResourceLimiter{
				Spec: rlv1beta2.ResourceLimiterSpec{
					Quotas:  unstructuredCR.Object["quotas"].([]rlv1beta2.ResourceLimiterQuota),
					Applied: unstructuredCR.Object["applied"].(bool),
				},
			}

			oldVer, status := convertV1beta2IntoV1beta1(cr)
			if status.Status != metav1.StatusSuccess {
				klog.Error(status.String())
				return &v1beta1.ConversionResponse{
					Result: status,
				}
			}
			convertedObjects = append(convertedObjects, runtime.RawExtension{Object: oldVer})
		}
	default:
		return &v1beta1.ConversionResponse{
			ConvertedObjects: convertedObjects,
			Result:           statusErrorWithMessage("failed to do the conversion"),
		}

	}

	return &v1beta1.ConversionResponse{
		ConvertedObjects: convertedObjects,
		Result:           statusSucceed(),
	}
}
