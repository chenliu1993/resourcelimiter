package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	rlv1beta1 "github.com/chenliu1993/resourcelimiter/api/v1beta1"
	rlv1beta2 "github.com/chenliu1993/resourcelimiter/api/v1beta2"
	"github.com/munnerz/goautoneg"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	// "k8s.io/apimachinery/pkg/runtime/serializer"
	k8sjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/klog"
)

var (
	runtimeScheme      = runtime.NewScheme()
	WebhookConvertPath = "/convert"
)

type WebhookServer struct {
	server *http.Server
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

			// This part is bound to how the v1beta1 is organized
			specObject := unstructuredCR.Object["spec"].(map[string]interface{})
			targets := []rlv1beta1.ResourceLimiterNamespace{}
			for _, item := range specObject["targets"].([]interface{}) {
				targets = append(targets, rlv1beta1.ResourceLimiterNamespace(item.(string)))
			}
			types := map[rlv1beta1.ResourceLimiterType]string{}
			for k, v := range specObject["types"].(map[string]interface{}) {
				types[rlv1beta1.ResourceLimiterType(k)] = v.(string)
			}

			cr := &rlv1beta1.ResourceLimiter{
				Spec: rlv1beta1.ResourceLimiterSpec{
					Applied: specObject["applied"].(bool),
					Targets: targets,
					Types:   types,
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

			// This part is bound to how the v1beta2 is organized
			specObject := unstructuredCR.Object["spec"].(map[string]interface{})
			quotas := []rlv1beta2.ResourceLimiterQuota{}
			for _, item := range specObject["targets"].([]interface{}) {
				itemMap := item.(map[string]interface{})
				quotas = append(quotas, rlv1beta2.ResourceLimiterQuota{
					NamespaceName: itemMap["name"].(string),
					CpuRequest:    itemMap["cpu_requests"].(string),
					CpuLimit:      itemMap["cpu_limits"].(string),
					MemRequest:    itemMap["mem_requests"].(string),
					MemLimit:      itemMap["mem_limits"].(string),
				})
			}

			cr := &rlv1beta2.ResourceLimiter{
				Spec: rlv1beta2.ResourceLimiterSpec{
					Quotas:  quotas,
					Applied: specObject["applied"].(bool),
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
