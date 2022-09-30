package main

import (
	"bytes"
	"context"
	"fmt"
	"reflect"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	webhookConfigName   = "resourcelimiter-checker"
	WebhookMutatePath   = "/mutate"
	WebhookValidatePath = "/validate"
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
							APIVersions: []string{"v1beta1"},
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
							APIVersions: []string{"v1beta1"},
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
