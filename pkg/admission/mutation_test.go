package admission

import (
	"bytes"
	"encoding/json"
	"github.com/google/go-cmp/cmp"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/runtime"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMutation(t *testing.T) {
	testCases := []struct {
		desc           string
		request        *admissionv1beta1.AdmissionReview
		response       *admissionv1beta1.AdmissionReview
		wantStatusCode int
		wantError      string
	}{
		{
			desc:           "nil body",
			wantError:      `Internal Server Error: "/secrets": unexpected nil request`,
			wantStatusCode: http.StatusInternalServerError,
		},
		{
			desc:           "empty AdmissionReview",
			request:        &admissionv1beta1.AdmissionReview{},
			wantStatusCode: http.StatusInternalServerError,
			wantError:      `Internal Server Error: "/secrets": unexpected nil request`,
		},
		{
			desc: "AdmissionReview with empty AdmissionRequest",
			request: &admissionv1beta1.AdmissionReview{
				Request: &admissionv1beta1.AdmissionRequest{},
			},
			response: &admissionv1beta1.AdmissionReview{
				Request: &admissionv1beta1.AdmissionRequest{},
				Response: &admissionv1beta1.AdmissionResponse{
					Result: &metav1.Status{
						Message: "Object 'Kind' is missing in ''",
						Status:  metav1.StatusFailure,
					},
				},
			},
			wantStatusCode: http.StatusOK,
		},
		{
			desc: "AdmissionRequest with empty Object",
			request: &admissionv1beta1.AdmissionReview{
				Request: &admissionv1beta1.AdmissionRequest{
					Object: runtime.RawExtension{},
				},
			},
			response: &admissionv1beta1.AdmissionReview{
				Request: &admissionv1beta1.AdmissionRequest{
					Object: runtime.RawExtension{},
				},
				Response: &admissionv1beta1.AdmissionResponse{
					Result: &metav1.Status{
						Message: "Object 'Kind' is missing in ''",
						Status:  metav1.StatusFailure,
					},
				},
			},
			wantStatusCode: http.StatusOK,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			var (
				requestBody  []byte
				responseBody []byte
				err          error
			)

			if tt.request != nil {
				requestBody, err = json.Marshal(tt.request)
				if err != nil {
					t.Fatalf("Failed to marshal AdmissionReview, err: %v", err)
				}
			}

			req := httptest.NewRequest(http.MethodGet, "/secrets", bytes.NewReader(requestBody))
			w := httptest.NewRecorder()
			Serve(w, req)

			resp := w.Result()
			if resp.StatusCode != tt.wantStatusCode {
				t.Fatalf("Got StatusCode %v want %v", resp.StatusCode, tt.wantStatusCode)
			}

			responseBody, err = ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read resp.Body, err: %v", err)
			}

			if resp.StatusCode != http.StatusOK && tt.wantError != strings.TrimSuffix(string(responseBody), "\n") {
				t.Fatalf("Expeted the body to contain an error. Got: %s want: %s", responseBody, tt.wantError)
			}

			// We don't expect to receive anything parsable to AdmissionReview in the body if an
			// internal error occurred, so we stop testing here.
			if resp.StatusCode == http.StatusInternalServerError {
				return
			}

			got := &admissionv1beta1.AdmissionReview{}
			err = json.Unmarshal(responseBody, got)
			if err != nil {
				t.Fatalf("Failed to unmarshal AdmissionReview, err: %v", err)
			}

			if diff := cmp.Diff(tt.response, got); diff != "" {
				t.Fatalf("Mismatch in AdmissionReview (-want, +got)\n%s", diff)
			}
		})
	}
}