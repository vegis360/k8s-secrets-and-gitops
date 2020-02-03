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

package admission

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/klog"
)

var (
	scheme    = runtime.NewScheme()
	codecs    = serializer.NewCodecFactory(scheme)
	reviewGVK = admissionv1beta1.SchemeGroupVersion.WithKind("AdmissionReview")
)

func init() {
	utilruntime.Must(admissionv1beta1.AddToScheme(scheme))
}

func Serve(w http.ResponseWriter, req *http.Request) {
	review, err := validateRequest(req)
	if err != nil {
		responsewriters.InternalError(w, req, err)
		return
	}

	review.Response = &admissionv1beta1.AdmissionResponse{
		UID: review.Request.UID,
	}

	// decode object
	if review.Request.Object.Object == nil {
		var err error
		review.Request.Object.Object, _, err = codecs.UniversalDeserializer().Decode(review.Request.Object.Raw, nil, nil)
		if err != nil {
			review.Response.Result = &metav1.Status{
				Message: err.Error(),
				Status:  metav1.StatusFailure,
			}
			writeReview(w, review)
			return
		}
	}

	// TODO(immutableT) check if review.Request.Object.Object == nil
	switch secret := review.Request.Object.Object.(type) {
	case *corev1.Secret:
		for k, v := range secret.Data {
			// TODO (immutableT) Add logic to detect encrypted values.
			// TODO (immutableT) Add logic to decrypt values.
			klog.Infof("Processing k:%v, v: %v", k, v)
		}

	default:
		responsewriters.InternalError(w, req, fmt.Errorf("unexpected object type: %v", secret))
		return
	}

	// TODO(immutableT) Generate Json path - this is what has to be attached to the response.
	// See github.com/appscode/jsonpatch or k8s.io/client-go/util/jsonpath/jsonpath

	review.Response.Allowed = true

	writeReview(w, review)
}

func validateRequest(req *http.Request) (*admissionv1beta1.AdmissionReview, error) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %v", err)
	}

	obj, gvk, err := codecs.UniversalDeserializer().Decode(body, &reviewGVK, &admissionv1beta1.AdmissionReview{})
	if err != nil {
		return nil, fmt.Errorf("failed to decode body: %v", err)
	}
	review, ok := obj.(*admissionv1beta1.AdmissionReview)
	if !ok {
		return nil, fmt.Errorf("unexpected GroupVersionKind: %s", gvk)
	}
	if review.Request == nil {
		return nil, errors.New("unexpected nil request")
	}
	return review, nil
}

// TODO(immutableT) This could handled more concisely with k8s.io/apiserver/pkg/endpoints/handlers/responsewriters
func writeReview(w http.ResponseWriter, review *admissionv1beta1.AdmissionReview) {
	// TODO(immutableT) This could handled more concisely with k8s.io/apiserver/pkg/endpoints/handlers/responsewriters
	resp, err := json.Marshal(review)
	if err != nil {
		klog.Errorf("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}

	if _, err := w.Write(resp); err != nil {
		klog.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}