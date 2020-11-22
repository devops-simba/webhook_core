package webhook_core

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/devops-simba/helpers"

	log "github.com/golang/glog"
	admissionApi "k8s.io/api/admission/v1"
	admissionApiBeta1 "k8s.io/api/admission/v1beta1"
	admissionRegistration "k8s.io/api/admissionregistration/v1"
	k8sCore "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

const jsonMIME = "application/json"
const verAdmissionApi = "admission.k8s.io/v1"
const verAdmissionApiBeta1 = "admission.k8s.io/v1beta1"

var (
	appliedSchemes []string
	// Scheme Default runtime scheme
	Scheme       = runtime.NewScheme()
	codecs       = serializer.NewCodecFactory(Scheme)
	deserializer = codecs.UniversalDeserializer()

	// (https://github.com/kubernetes/kubernetes/issues/57982)
	defaulter = runtime.ObjectDefaulter(Scheme)
)

func v1tobeta1Operation(source admissionApi.Operation) admissionApiBeta1.Operation {
	return admissionApiBeta1.Operation(string(source))
}
func v1tobeta1AdmissionRequest(source *admissionApi.AdmissionRequest) *admissionApiBeta1.AdmissionRequest {
	if source == nil {
		return nil
	}
	return &admissionApiBeta1.AdmissionRequest{
		UID:                source.UID,
		Kind:               source.Kind,
		Resource:           source.Resource,
		SubResource:        source.SubResource,
		RequestKind:        source.RequestKind,
		RequestResource:    source.RequestResource,
		RequestSubResource: source.RequestSubResource,
		Name:               source.Name,
		Namespace:          source.Namespace,
		Operation:          v1tobeta1Operation(source.Operation),
		UserInfo:           source.UserInfo,
		Object:             source.Object,
		OldObject:          source.OldObject,
		DryRun:             source.DryRun,
		Options:            source.Options,
	}
}
func v1tobeta1PathType(source *admissionApi.PatchType) *admissionApiBeta1.PatchType {
	if source == nil {
		return nil
	}

	value := admissionApiBeta1.PatchType(string(*source))
	return &value
}
func v1tobeta1AdmissionResponse(source *admissionApi.AdmissionResponse) *admissionApiBeta1.AdmissionResponse {
	if source == nil {
		return nil
	}
	return &admissionApiBeta1.AdmissionResponse{
		UID:              source.UID,
		Allowed:          source.Allowed,
		Result:           source.Result,
		Patch:            source.Patch,
		PatchType:        v1tobeta1PathType(source.PatchType),
		AuditAnnotations: source.AuditAnnotations,
	}
}
func v1tobeta1AdmissionReview(source *admissionApi.AdmissionReview) *admissionApiBeta1.AdmissionReview {
	if source == nil {
		return nil
	}
	return &admissionApiBeta1.AdmissionReview{
		Request:  v1tobeta1AdmissionRequest(source.Request),
		Response: v1tobeta1AdmissionResponse(source.Response),
	}
}

func beta1tov1Operation(source admissionApiBeta1.Operation) admissionApi.Operation {
	return admissionApi.Operation(string(source))
}
func beta1tov1AdmissionRequest(source *admissionApiBeta1.AdmissionRequest) *admissionApi.AdmissionRequest {
	if source == nil {
		return nil
	}
	return &admissionApi.AdmissionRequest{
		UID:                source.UID,
		Kind:               source.Kind,
		Resource:           source.Resource,
		SubResource:        source.SubResource,
		RequestKind:        source.RequestKind,
		RequestResource:    source.RequestResource,
		RequestSubResource: source.RequestSubResource,
		Name:               source.Name,
		Namespace:          source.Namespace,
		Operation:          beta1tov1Operation(source.Operation),
		UserInfo:           source.UserInfo,
		Object:             source.Object,
		OldObject:          source.OldObject,
		DryRun:             source.DryRun,
		Options:            source.Options,
	}
}
func beta1tov1PathType(source *admissionApiBeta1.PatchType) *admissionApi.PatchType {
	if source == nil {
		return nil
	}

	value := admissionApi.PatchType(string(*source))
	return &value
}
func beta1tov1AdmissionResponse(source *admissionApiBeta1.AdmissionResponse) *admissionApi.AdmissionResponse {
	if source == nil {
		return nil
	}
	return &admissionApi.AdmissionResponse{
		UID:              source.UID,
		Allowed:          source.Allowed,
		Result:           source.Result,
		Patch:            source.Patch,
		PatchType:        beta1tov1PathType(source.PatchType),
		AuditAnnotations: source.AuditAnnotations,
	}
}
func beta1tov1AdmissionReview(source *admissionApiBeta1.AdmissionReview) *admissionApi.AdmissionReview {
	if source == nil {
		return nil
	}
	return &admissionApi.AdmissionReview{
		Request:  beta1tov1AdmissionRequest(source.Request),
		Response: beta1tov1AdmissionResponse(source.Response),
	}
}

// ReadAdmissionReview read an AdmissionReview from a request
func ReadAdmissionReview(request *http.Request) (string, *admissionApi.AdmissionReview, error) {
	var body []byte
	if request.Body != nil {
		data, err := ioutil.ReadAll(request.Body)
		if err != nil {
			return "", nil, err
		}
		body = data
	}

	if log.V(8) {
		bodyString := string(body)
		log.Infof("Request body: %v", bodyString)
	}

	if len(body) == 0 {
		return "", nil, fmt.Errorf("Empty body")
	}

	contentType := request.Header.Get("Content-Type")
	if contentType != jsonMIME {
		return "", nil, fmt.Errorf("Invalid content-type. Received: %v, Expected: %v", jsonMIME, contentType)
	}

	var apiVersion string
	var ar *admissionApi.AdmissionReview

	// try to work with v1
	arV1 := admissionApi.AdmissionReview{}
	_, _, err := deserializer.Decode(body, nil, &arV1)

	if err != nil {
		// try with beta1, test if this is a beta1 admission
		arBeta1 := admissionApiBeta1.AdmissionReview{}
		_, _, err = deserializer.Decode(body, nil, &arBeta1)
		if err != nil {
			return "", nil, err
		}

		ar = beta1tov1AdmissionReview(&arBeta1)
		apiVersion = verAdmissionApiBeta1
	} else {
		ar = &arV1
		apiVersion = verAdmissionApi
	}

	return apiVersion, ar, err
}

// CreateErrorResponse create an admission error response
func CreateErrorResponse(errorDesc string) *admissionApi.AdmissionResponse {
	return &admissionApi.AdmissionResponse{
		Result: &metav1.Status{
			Status:  "Failure",
			Message: errorDesc,
		},
	}
}

// WriteAdmissionResponse write an AdmissionResponse as a HTTP response
func WriteAdmissionResponse(
	writer http.ResponseWriter,
	apiVersion string,
	ar *admissionApi.AdmissionReview,
	response *admissionApi.AdmissionResponse) {
	responseAR := admissionApi.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AdmissionReview",
			APIVersion: apiVersion,
		},
	}
	if response != nil {
		responseAR.Response = response
		if ar.Request != nil {
			responseAR.Response.UID = ar.Request.UID
		}
	}

	var err error
	var resp []byte
	if apiVersion == verAdmissionApiBeta1 {
		arBeta1 := v1tobeta1AdmissionReview(&responseAR)
		resp, err = json.Marshal(&arBeta1)
	} else {
		resp, err = json.Marshal(responseAR)
	}

	if err != nil {
		log.Errorf("Can't encode the response as JSON: %v", err)
		http.Error(writer, "Failed to encode response", http.StatusInternalServerError)
	} else if _, err := writer.Write(resp); err != nil {
		log.Errorf("Failed to write response: %v", err)
		http.Error(writer, "Failed to send response", http.StatusInternalServerError)
	} else {
		log.V(10).Infof("Sent response: %v", string(resp))
	}
}

// InitializeRuntimeScheme initialize a runtime scheme
// this is useful for adding schemes from different APIs(like openshift)
func InitializeRuntimeScheme(partName string, updater func(*runtime.Scheme) error) error {
	if !helpers.ContainsString(appliedSchemes, partName) {
		err := updater(Scheme)
		if err != nil {
			return err
		}
		appliedSchemes = append(appliedSchemes, partName)
	}
	return nil
}

func init() {
	_ = InitializeRuntimeScheme("k8s.io/api/core/v1", k8sCore.AddToScheme)
	_ = InitializeRuntimeScheme("k8s.io/api/admission/v1", admissionApi.AddToScheme)
	_ = InitializeRuntimeScheme("k8s.io/api/admissionregistration/v1", admissionRegistration.AddToScheme)
}
