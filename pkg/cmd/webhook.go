package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	rlv1beta1 "github.com/chenliu1993/resourcelimiter/api/v1beta1"
	"github.com/chenliu1993/resourcelimiter/pkg/constants"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
)

// var ignoredNamespaces = []string{
// 	metav1.NamespaceSystem,
// 	metav1.NamespacePublic,
// }

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
func mutationRequired(rl *rlv1beta1.ResourceLimiter) (bool, bool) {
	var requiredTypes, requiredTargets bool
	if len(rl.Spec.Targets) == 0 {
		requiredTargets = true
	}

	if len(rl.Spec.Types) == 0 {
		requiredTypes = true
	}

	infoLogger.Printf("Mutation policy for %v required:%v", rl.Name, requiredTypes || requiredTargets)
	return requiredTypes, requiredTargets
}

func updateResourceLimiterTypes(target map[rlv1beta1.ResourceLimiterType]string, added map[rlv1beta1.ResourceLimiterType]string) (patch []patchOperation) {
	for key, value := range added {
		if len(target) == 0 {
			target = map[rlv1beta1.ResourceLimiterType]string{}
			patch = append(patch, patchOperation{
				Op:   "add",
				Path: "/spec/types",
				Value: map[rlv1beta1.ResourceLimiterType]string{
					key: value,
				},
			})
		} else {
			patch = append(patch, patchOperation{
				Op:    "replace",
				Path:  "/spec/types/" + string(key),
				Value: value,
			})
		}
	}
	return patch
}

func updateResourceLimiterTargets(target []rlv1beta1.ResourceLimiterNamespace, added []rlv1beta1.ResourceLimiterNamespace) (patch []patchOperation) {
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
				Path:  "/spec/types/-",
				Value: item,
			})
		}
	}
	return patch
}

// create mutation patch for resoures
func createPatch(rl *rlv1beta1.ResourceLimiter, desired *rlv1beta1.ResourceLimiter) ([]byte, error) {
	var patch []patchOperation

	requiredTypes, requiredTargets := mutationRequired(rl)
	if requiredTypes {
		patch = append(patch, updateResourceLimiterTypes(rl.Spec.Types, desired.Spec.Types)...)
	}

	if requiredTargets {
		patch = append(patch, updateResourceLimiterTargets(rl.Spec.Targets, desired.Spec.Targets)...)
	}

	return json.Marshal(patch)
}

// main mutation process
func (whsvr *WebhookServer) mutate(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	req := ar.Request
	var rl rlv1beta1.ResourceLimiter
	if err := json.Unmarshal(req.Object.Raw, &rl); err != nil {
		warningLogger.Printf("Could not unmarshal raw object: %v", err)
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	infoLogger.Printf("AdmissionReview for Kind=%v, Namespace=%v Name=%v (%v) UID=%v patchOperation=%v UserInfo=%v",
		req.Kind, req.Namespace, req.Name, rl.Name, req.UID, req.Operation, req.UserInfo)

	desired := rlv1beta1.ResourceLimiter{
		Spec: rlv1beta1.ResourceLimiterSpec{
			Types: map[rlv1beta1.ResourceLimiterType]string{constants.RetrainTypeLimitsCpu: "2", constants.RetrainTypeLimitsMemory: "200Mi",
				constants.RetrainTypeRequestsCpu: "1", constants.RetrainTypeRequestsMemory: "150Mi"},
			Targets: []rlv1beta1.ResourceLimiterNamespace{"default"},
		},
	}
	patchBytes, err := createPatch(&rl, &desired)
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

func recordR(log *log.Logger, er error) {
	if err := recover(); err != nil {
		log.Printf(fmt.Sprintf("MustParse failed due to %v", err))
		er = fmt.Errorf("MustParse failed due to %v", err)
	}
}

func (whsvr *WebhookServer) validate(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	req := ar.Request
	var (
		rl    rlv1beta1.ResourceLimiter
		er    error
		pod   corev1.Pod
		isPod bool
	)

	defer recordR(warningLogger, er)

	if err := json.Unmarshal(req.Object.Raw, &rl); err != nil {
		warningLogger.Printf("Could not unmarshal raw object into resourcelimiter, try pod: %v", err)
		if er := json.Unmarshal(req.Object.Raw, &pod); er != nil {
			warningLogger.Printf("Could not unmarshal raw object into pod either: %v", err)
			return &admissionv1.AdmissionResponse{
				Result: &metav1.Status{
					Message: err.Error(),
				},
			}
		} else {
			infoLogger.Printf("Marshalled into pod")
			isPod = true
		}
	} else {
		infoLogger.Printf("Marshalled into resourcelimiter")
		isPod = false
	}

	infoLogger.Printf("AdmissionReview for Kind=%v, Namespace=%v Name=%v (%v) UID=%v patchOperation=%v UserInfo=%v",
		req.Kind, req.Namespace, req.Name, rl.Name, req.UID, req.Operation, req.UserInfo)

	if isPod {
		// Ignore init and ehperamal
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
	} else {
		for t, value := range rl.Spec.Types {
			warningLogger.Printf(fmt.Sprintf("validating type field %s for %s", t, rl.Name))
			k8sresource.MustParse(value)
		}
	}

	if er != nil {
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: er.Error(),
			},
		}
	}

	return &admissionv1.AdmissionResponse{
		Allowed: true,
		Result: &metav1.Status{
			Message: "Validate OK",
		},
	}
}

// Serve method for webhook server
func (whsvr *WebhookServer) ServeMutate(w http.ResponseWriter, r *http.Request) {
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
		admissionResponse = whsvr.validate(&ar)
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
