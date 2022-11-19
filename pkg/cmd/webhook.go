package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	rlv1beta1 "github.com/chenliu1993/resourcelimiter/api/v1beta1"
	rlv1beta2 "github.com/chenliu1993/resourcelimiter/api/v1beta2"
	"github.com/chenliu1993/resourcelimiter/pkg/constants"
	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
	resFormErr    error
)

// const (
// 	admissionWebhookAnnotationMutatewKey  = "resourcelimiter.cliufreever.io/mutate"
// 	admissionWebhookAnnotationValidateKey = "resourcelimiter.cliufreever.io/validate"
// )

type WebhookServer struct {
	server *http.Server
}

// Webhook Server parameters
// type WhSvrParameters struct {
// 	port     int    // webhook server port
// 	certFile string // path to the x509 certificate for https
// 	keyFile  string // path to the x509 private key matching `CertFile`
// }

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

// Check whether the target resoured need to be mutated
func mutationRequiredV1beta1(rl *rlv1beta1.ResourceLimiter) (bool, bool) {
	var requiredTypes, requiredTargets bool
	if len(rl.Spec.Targets) == 0 {
		requiredTargets = true
	}

	if len(rl.Spec.Types) == 0 {
		requiredTypes = true
	}

	if _, ok := rl.Spec.Types[constants.RetrainTypeLimitsCpu]; !ok {
		requiredTypes = true
	}
	if _, ok := rl.Spec.Types[constants.RetrainTypeLimitsMemory]; !ok {
		requiredTypes = true
	}
	if _, ok := rl.Spec.Types[constants.RetrainTypeRequestsCpu]; !ok {
		requiredTypes = true
	}
	if _, ok := rl.Spec.Types[constants.RetrainTypeRequestsMemory]; !ok {
		requiredTypes = true
	}

	infoLogger.Printf("Mutation policy for v1beta1/%v required:%v", rl.Name, requiredTypes || requiredTargets)
	return requiredTypes, requiredTargets
}

func updateResourceLimiterTypesV1beta1(added map[rlv1beta1.ResourceLimiterType]string) (patch []patchOperation) {
	patch = append(patch, patchOperation{
		Op:    "add",
		Path:  "/spec/types",
		Value: added,
	})
	return patch
}

func updateResourceLimiterTargetsV1beta1(target []rlv1beta1.ResourceLimiterNamespace, added []rlv1beta1.ResourceLimiterNamespace) (patch []patchOperation) {
	for _, item := range added {
		if len(target) == 0 {
			target = []rlv1beta1.ResourceLimiterNamespace{}
			patch = append(patch, patchOperation{
				Op:   "add",
				Path: "/spec/targets",
				Value: []rlv1beta1.ResourceLimiterNamespace{
					item,
				},
			})
		} else {
			patch = append(patch, patchOperation{
				Op:    "add",
				Path:  "/spec/targets/-",
				Value: item,
			})
		}
	}
	return patch
}

// create mutation patch for resoures
func createPatchV1beta1(rl *rlv1beta1.ResourceLimiter, desired *rlv1beta1.ResourceLimiter) ([]byte, error) {
	var patch []patchOperation

	requiredTypes, requiredTargets := mutationRequiredV1beta1(rl)
	if requiredTypes {
		patch = append(patch, updateResourceLimiterTypesV1beta1(desired.Spec.Types)...)
	}

	if requiredTargets {
		patch = append(patch, updateResourceLimiterTargetsV1beta1(rl.Spec.Targets, desired.Spec.Targets)...)
	}

	return json.Marshal(patch)
}

// Check whether the target resoured need to be mutated
func mutationRequiredV1beta2(rl *rlv1beta2.ResourceLimiter) map[string]bool {

	if len(rl.Spec.Quotas) == 0 {
		return map[string]bool{
			"default": true,
		}
	}

	requiredQuotas := map[string]bool{}
	for _, v := range rl.Spec.Quotas {
		if v.CpuLimit == "" || v.CpuRequest == "" || v.MemLimit == "" || v.MemRequest == "" {
			// This ns should be mutated
			requiredQuotas[v.NamespaceName] = true
		}
	}

	infoLogger.Printf("Mutation policy for v1beta2/%v required:%v", rl.Name, requiredQuotas)
	return requiredQuotas
}

func updateResourceLimiterQuotasV1beta2(target, added []rlv1beta2.ResourceLimiterQuota) (patch []patchOperation) {
	for _, item := range added {
		if len(target) == 0 {
			target = []rlv1beta2.ResourceLimiterQuota{}
			patch = append(patch, patchOperation{
				Op:   "add",
				Path: "/spec/targets",
				Value: []rlv1beta2.ResourceLimiterQuota{
					item,
				},
			})
		} else {
			patch = append(patch, patchOperation{
				Op:    "add",
				Path:  "/spec/targets/-",
				Value: item,
			})
		}
	}
	return patch
}

// create mutation patch for resoures
func createPatchV1beta2(rl *rlv1beta2.ResourceLimiter, desired *rlv1beta2.ResourceLimiter) ([]byte, error) {
	var patch []patchOperation

	requiredQuotas := mutationRequiredV1beta2(rl)
	// TODO: better  find
	for k, v := range requiredQuotas {
		for _, quota := range desired.Spec.Quotas {
			if v && k == quota.NamespaceName {
				patch = append(patch, updateResourceLimiterQuotasV1beta2(rl.Spec.Quotas, []rlv1beta2.ResourceLimiterQuota{
					// Quotas.NamespaceName must exists, because desired is generated from the mutate-needed spec
					quota,
				})...)
			}
		}
	}
	return json.Marshal(patch)
}

func setDesired(rl *rlv1beta2.ResourceLimiter) *admissionv1.AdmissionResponse {
	desired := rlv1beta2.ResourceLimiter{
		Spec: rlv1beta2.ResourceLimiterSpec{
			Quotas: []rlv1beta2.ResourceLimiterQuota{},
		},
	}

	for _, item := range rl.Spec.Quotas {
		// We set all to default
		// TODO maybe later I can try only change the problem field
		desired.Spec.Quotas = append(desired.Spec.Quotas, rlv1beta2.ResourceLimiterQuota{
			NamespaceName: item.NamespaceName,
			CpuRequest:    "1",
			CpuLimit:      "2",
			MemRequest:    "150Mi",
			MemLimit:      "200Mi",
		})
	}

	patchBytes, err := createPatchV1beta2(rl, &desired)
	if err != nil {
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	infoLogger.Printf("AdmissionResponse: patch=%v\n", string(patchBytes))
	return &admissionv1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

// main mutation process
func (whsvr *WebhookServer) mutate(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	req := ar.Request
	switch req.Kind.Version {
	case "v1beta1":
		var rl rlv1beta1.ResourceLimiter
		if err := json.Unmarshal(req.Object.Raw, &rl); err != nil {
			warningLogger.Printf("Could not unmarshal raw object: %v", err)
			return &admissionv1.AdmissionResponse{
				Result: &metav1.Status{
					Message: err.Error(),
				},
			}
		}

		infoLogger.Printf("Mutate AdmissionReview for Kind=%v, Namespace=%v Name=%v (%v) UID=%v patchOperation=%v UserInfo=%v",
			req.Kind, req.Namespace, req.Name, rl.Name, req.UID, req.Operation, req.UserInfo)

		desired := rlv1beta1.ResourceLimiter{
			Spec: rlv1beta1.ResourceLimiterSpec{
				Types: map[rlv1beta1.ResourceLimiterType]string{constants.RetrainTypeLimitsCpu: "2", constants.RetrainTypeLimitsMemory: "200Mi",
					constants.RetrainTypeRequestsCpu: "1", constants.RetrainTypeRequestsMemory: "150Mi"},
				Targets: []rlv1beta1.ResourceLimiterNamespace{"default"},
			},
		}
		patchBytes, err := createPatchV1beta1(&rl, &desired)
		if err != nil {
			return &admissionv1.AdmissionResponse{
				Result: &metav1.Status{
					Message: err.Error(),
				},
			}
		}

		infoLogger.Printf("AdmissionResponse: patch=%v\n", string(patchBytes))
		return &admissionv1.AdmissionResponse{
			Allowed: true,
			Patch:   patchBytes,
			PatchType: func() *admissionv1.PatchType {
				pt := admissionv1.PatchTypeJSONPatch
				return &pt
			}(),
		}
	case "v1beta2":
		var rl rlv1beta2.ResourceLimiter
		if err := json.Unmarshal(req.Object.Raw, &rl); err != nil {
			warningLogger.Printf("Could not unmarshal raw object: %v", err)
			return &admissionv1.AdmissionResponse{
				Result: &metav1.Status{
					Message: err.Error(),
				},
			}
		}

		infoLogger.Printf("Mutate AdmissionReview for Kind=%v, Namespace=%v Name=%v (%v) UID=%v patchOperation=%v UserInfo=%v",
			req.Kind, req.Namespace, req.Name, rl.Name, req.UID, req.Operation, req.UserInfo)

		desired := rlv1beta2.ResourceLimiter{
			Spec: rlv1beta2.ResourceLimiterSpec{
				Quotas: []rlv1beta2.ResourceLimiterQuota{},
			},
		}

		for ns := range rl.Spec.Quotas {
			// We set all to default
			// TODO maybe later I can try only change the problem field
			desired.Spec.Quotas[ns] = rlv1beta2.ResourceLimiterQuota{
				CpuRequest: "1",
				CpuLimit:   "2",
				MemRequest: "150Mi",
				MemLimit:   "200Mi",
			}
		}

		patchBytes, err := createPatchV1beta2(&rl, &desired)
		if err != nil {
			return &admissionv1.AdmissionResponse{
				Result: &metav1.Status{
					Message: err.Error(),
				},
			}
		}

		infoLogger.Printf("AdmissionResponse: patch=%v\n", string(patchBytes))
		return &admissionv1.AdmissionResponse{
			Allowed: true,
			Patch:   patchBytes,
			PatchType: func() *admissionv1.PatchType {
				pt := admissionv1.PatchTypeJSONPatch
				return &pt
			}(),
		}
	}
	return &admissionv1.AdmissionResponse{
		Result: &metav1.Status{
			Message: fmt.Sprintf("Unsupported version %s", req.Kind.Version),
		},
	}
}

func recordR(log *log.Logger) {
	if err := recover(); err != nil {
		log.Printf(fmt.Sprintf("MustParse failed due to %v", err))
		resFormErr = fmt.Errorf("MustParse failed due to %v", err)
	}
}

func (whsvr *WebhookServer) validate(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	req := ar.Request
	resFormErr = nil
	defer recordR(warningLogger)

	switch req.Kind.Kind {
	case "ResourceLimiter":
		switch req.Kind.Version {
		case "v1beta1":
			var rl rlv1beta1.ResourceLimiter
			infoLogger.Printf("begin marshal resourcelimiter %s of %s", req.Name, req.Kind.Kind)
			if err := json.Unmarshal(req.Object.Raw, &rl); err != nil {
				warningLogger.Printf("Could not unmarshal raw object into resourcelimiter, try pod: %v", err)
				return &admissionv1.AdmissionResponse{
					Allowed: false,
					Result: &metav1.Status{
						Message: err.Error(),
					},
				}
			}
			infoLogger.Printf("Validate AdmissionReview for Kind=%v, Namespace=%v Name=%v (%v) UID=%v patchOperation=%v UserInfo=%v",
				req.Kind, req.Namespace, req.Name, rl.Name, req.UID, req.Operation, req.UserInfo)
			for _, ns := range rl.Spec.Targets {
				if ns == constants.IgnoreKubeSystem || ns == constants.IgnoreKubePublic {
					return &admissionv1.AdmissionResponse{
						Allowed: false,
						Result: &metav1.Status{
							Message: "should avoid limit on the preset namespace",
						},
					}
				}
			}
			for t, value := range rl.Spec.Types {
				warningLogger.Printf(fmt.Sprintf("validating type field %s for %s", t, rl.Name))
				k8sresource.MustParse(value)
			}
		case "v1beta2":
			var rl rlv1beta2.ResourceLimiter
			infoLogger.Printf("begin marshal resourcelimiter v1beta2/%s of %s", req.Name, req.Kind.Kind)
			if err := json.Unmarshal(req.Object.Raw, &rl); err != nil {
				warningLogger.Printf("Could not unmarshal raw object into resourcelimiter, try pod: %v", err)
				return &admissionv1.AdmissionResponse{
					Allowed: false,
					Result: &metav1.Status{
						Message: err.Error(),
					},
				}
			}
			infoLogger.Printf("Validate AdmissionReview for Kind=%v, Namespace=%v Name=%v (%v) UID=%v patchOperation=%v UserInfo=%v",
				req.Kind, req.Namespace, req.Name, rl.Name, req.UID, req.Operation, req.UserInfo)

			for _, quota := range rl.Spec.Quotas {
				if quota.NamespaceName == string(constants.IgnoreKubeSystem) || quota.NamespaceName == string(constants.IgnoreKubePublic) {
					return &admissionv1.AdmissionResponse{
						Allowed: false,
						Result: &metav1.Status{
							Message: "should avoid limit on the preset namespace",
						},
					}
				}
				warningLogger.Printf(fmt.Sprintf("validating quota field CpuLimitfor for %s", rl.Name))
				k8sresource.MustParse(quota.CpuLimit)
				warningLogger.Printf(fmt.Sprintf("validating quota field CpuRequest for %s", rl.Name))
				k8sresource.MustParse(quota.CpuRequest)
				warningLogger.Printf(fmt.Sprintf("validating quota field MemLimit for %s", rl.Name))
				k8sresource.MustParse(quota.MemLimit)
				warningLogger.Printf(fmt.Sprintf("validating quota field MemRequest for %s", rl.Name))
				k8sresource.MustParse(quota.MemRequest)
			}
		}

	case "Pod":
		var pod corev1.Pod
		infoLogger.Printf("begin marshal pod %s of %s", req.Name, req.Kind.Kind)
		if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
			warningLogger.Printf("Could not unmarshal raw object into pod either: %v", err)
			return &admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Message: err.Error(),
				},
			}
		}
		infoLogger.Printf("Validate AdmissionReview for Kind=%v, Namespace=%v Name=%v UID=%v patchOperation=%v UserInfo=%v",
			req.Kind, req.Namespace, req.Name, req.UID, req.Operation, req.UserInfo)
		for _, cont := range pod.Spec.Containers {
			if cont.Resources.Limits == nil || len(cont.Resources.Limits) == 0 || cont.Resources.Requests == nil || len(cont.Resources.Requests) == 0 {
				return &admissionv1.AdmissionResponse{
					Allowed: false,
					Result: &metav1.Status{
						Message: fmt.Sprintf("failed to validate pod %s not set any resources limits or requests", pod.Name),
					},
				}
			}
			if cont.Resources.Limits != nil {
				k8sresource.MustParse(cont.Resources.Limits.Cpu().String())
				k8sresource.MustParse(cont.Resources.Limits.Memory().String())
			}

			if cont.Resources.Requests != nil {
				k8sresource.MustParse(cont.Resources.Requests.Cpu().String())
				k8sresource.MustParse(cont.Resources.Requests.Memory().String())
			}

		}
	case "Deployment":
		var deployment appsv1.Deployment
		infoLogger.Printf("begin marshal deployment %s of %s", req.Name, req.Kind.Kind)
		if err := json.Unmarshal(req.Object.Raw, &deployment); err != nil {
			warningLogger.Printf("Could not unmarshal raw object into deployment either: %v", err)
			return &admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Message: err.Error(),
				},
			}
		}
		infoLogger.Printf("Validate AdmissionReview for Kind=%v, Namespace=%v Name=%v UID=%v patchOperation=%v UserInfo=%v",
			req.Kind, req.Namespace, req.Name, req.UID, req.Operation, req.UserInfo)
		for _, cont := range deployment.Spec.Template.Spec.Containers {
			if cont.Resources.Limits == nil || len(cont.Resources.Limits) == 0 || cont.Resources.Requests == nil || len(cont.Resources.Requests) == 0 {
				return &admissionv1.AdmissionResponse{
					Allowed: false,
					Result: &metav1.Status{
						Message: fmt.Sprintf("failed to validate deployment %s not set any resources limits or requests", deployment.Name),
					},
				}
			}
			if cont.Resources.Limits != nil {
				k8sresource.MustParse(cont.Resources.Limits.Cpu().String())
				k8sresource.MustParse(cont.Resources.Limits.Memory().String())
			}

			if cont.Resources.Requests != nil {
				k8sresource.MustParse(cont.Resources.Requests.Cpu().String())
				k8sresource.MustParse(cont.Resources.Requests.Memory().String())
			}
		}
	case "Daemonset":
		var daemonset appsv1.DaemonSet
		infoLogger.Printf("begin marshal daemonset %s of %s", req.Name, req.Kind.Kind)
		if err := json.Unmarshal(req.Object.Raw, &daemonset); err != nil {
			warningLogger.Printf("Could not unmarshal raw object into daemonset either: %v", err)
			return &admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Message: err.Error(),
				},
			}
		}
		infoLogger.Printf("Validate AdmissionReview for Kind=%v, Namespace=%v Name=%v UID=%v patchOperation=%v UserInfo=%v",
			req.Kind, req.Namespace, req.Name, req.UID, req.Operation, req.UserInfo)
		for _, cont := range daemonset.Spec.Template.Spec.Containers {
			if cont.Resources.Limits == nil || len(cont.Resources.Limits) == 0 || cont.Resources.Requests == nil || len(cont.Resources.Requests) == 0 {
				return &admissionv1.AdmissionResponse{
					Allowed: false,
					Result: &metav1.Status{
						Message: fmt.Sprintf("failed to validate daemonset %s not set any resources limits or requests", daemonset.Name),
					},
				}
			}
			if cont.Resources.Limits != nil {
				k8sresource.MustParse(cont.Resources.Limits.Cpu().String())
				k8sresource.MustParse(cont.Resources.Limits.Memory().String())
			}

			if cont.Resources.Requests != nil {
				k8sresource.MustParse(cont.Resources.Requests.Cpu().String())
				k8sresource.MustParse(cont.Resources.Requests.Memory().String())
			}
		}
	default:
		warningLogger.Printf("should not been here")
	}

	return &admissionv1.AdmissionResponse{
		Allowed: true,
		Result: &metav1.Status{
			Message: fmt.Sprintf("Validate %s of %s OK", req.Name, req.Kind.Kind),
		},
	}
}

// Serve method for webhook server
func (whsvr *WebhookServer) ServeMutate(w http.ResponseWriter, r *http.Request) {
	infoLogger.Printf("begin mutating webhook check")
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		warningLogger.Println("empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		warningLogger.Printf("Content-Type=%s, expect application/json", contentType)
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	var admissionResponse *admissionv1.AdmissionResponse
	ar := admissionv1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		warningLogger.Printf("Can't decode body: %v", err)
		admissionResponse = &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		admissionResponse = whsvr.mutate(&ar)
	}

	admissionReview := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
	}
	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
		if ar.Request != nil {
			admissionReview.Response.UID = ar.Request.UID
		}
	}

	resp, err := json.Marshal(admissionReview)
	if err != nil {
		warningLogger.Printf("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}
	infoLogger.Printf("Ready to write reponse ...")
	if _, err := w.Write(resp); err != nil {
		warningLogger.Printf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}

// Serve method for webhook server
func (whsvr *WebhookServer) ServeValidate(w http.ResponseWriter, r *http.Request) {
	infoLogger.Printf("begin validating webhook check")
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		warningLogger.Println("empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		warningLogger.Printf("Content-Type=%s, expect application/json", contentType)
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	var admissionResponse *admissionv1.AdmissionResponse
	ar := admissionv1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		warningLogger.Printf("Can't decode body: %v", err)
		admissionResponse = &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		admissionResponse = whsvr.validate(&ar)
	}

	// For parse panic
	if resFormErr != nil {
		warningLogger.Printf("failed to parse resources fields")
		admissionResponse = &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: resFormErr.Error(),
			},
		}
	}

	admissionReview := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
	}
	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
		if ar.Request != nil {
			admissionReview.Response.UID = ar.Request.UID
		}
	}

	resp, err := json.Marshal(admissionReview)
	if err != nil {
		warningLogger.Printf("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}
	infoLogger.Printf("Ready to write reponse ...")
	if _, err := w.Write(resp); err != nil {
		warningLogger.Printf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}

func convertV1beta1IntoV1beta2(oldObject *rlv1beta1.ResourceLimiter) (*rlv1beta2.ResourceLimiter, metav1.Status) {
	infoLogger.Printf("begin converting into v1beta2")
	fromVersion := "resources.resourcelimiter.io/v1beta1"
	toVersion := "resources.resourcelimiter.io/v1beta2"

	if toVersion == fromVersion {
		return nil, statusErrorWithMessage("conversion from a version to itself should not call the webhook: %s", toVersion)
	}

	newObject := &rlv1beta2.ResourceLimiter{}

	if err := oldObject.ConvertTo(newObject); err != nil {
		return nil, statusErrorWithMessage("failed to convert from %q into %q", fromVersion, toVersion)
	}
	return newObject, statusSucceed()
}

func convertV1beta2IntoV1beta1(newObject *rlv1beta2.ResourceLimiter) (*rlv1beta1.ResourceLimiter, metav1.Status) {
	infoLogger.Printf("begin converting into v1beta1")
	fromVersion := "resources.resourcelimiter.io/v1beta2"
	toVersion := "resources.resourcelimiter.io/v1beta1"

	if toVersion == fromVersion {
		return nil, statusErrorWithMessage("conversion from a version to itself should not call the webhook: %s", toVersion)
	}

	oldObject := &rlv1beta1.ResourceLimiter{}

	if err := oldObject.ConvertFrom(newObject); err != nil {
		return nil, statusErrorWithMessage("failed to convert from %q into %q", fromVersion, toVersion)
	}
	return oldObject, statusSucceed()
}

func (whsvr *WebhookServer) serveConvert(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	contentType := r.Header.Get("Content-Type")
	serializer := getInputSerializer(contentType)
	if serializer == nil {
		msg := fmt.Sprintf("invalid Content-Type header `%s`", contentType)
		warningLogger.Printf(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	infoLogger.Printf("handling request: %v", body)
	convertReview := v1beta1.ConversionReview{}
	if _, _, err := serializer.Decode(body, nil, &convertReview); err != nil {
		warningLogger.Printf(err.Error())
		convertReview.Response = conversionResponseFailureWithMessagef("failed to deserialize body (%v) with error %v", string(body), err)
	} else {
		convertReview.Response = doConversion(convertReview.Request)
		convertReview.Response.UID = convertReview.Request.UID
	}
	infoLogger.Printf(fmt.Sprintf("sending response: %v", convertReview.Response))

	// reset the request, it is not needed in a response.
	convertReview.Request = &v1beta1.ConversionRequest{}

	accept := r.Header.Get("Accept")
	outSerializer := getOutputSerializer(accept)
	if outSerializer == nil {
		msg := fmt.Sprintf("invalid accept header `%s`", accept)
		warningLogger.Printf(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	err := outSerializer.Encode(&convertReview, w)
	if err != nil {
		warningLogger.Printf(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (whsvr *WebhookServer) ServeConvert(w http.ResponseWriter, r *http.Request) {
	infoLogger.Printf("begin convertin webhook")
	whsvr.serveConvert(w, r)
}
