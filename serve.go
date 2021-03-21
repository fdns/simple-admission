package main

import (
	"encoding/json"
	"fmt"
	"log"
	"io/ioutil"
	"net/http"

	admission "k8s.io/api/admission/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	k8meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AdmissionHandler struct {
	RuntimeClass string
}

func (handler *AdmissionHandler) handler(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		data, err := ioutil.ReadAll(r.Body);
		if err == nil {
			body = data
		} else {
			log.Printf("Error %v", err)
			http.Error(w, "Error reading body", http.StatusBadRequest)
			return
		}
	}
	if len(body) == 0 {
		log.Printf("Body is empty")
		http.Error(w, "Body is empty", http.StatusBadRequest)
		return
	}

	log.Printf("Request: %v", string(body))
	request := admission.AdmissionReview{}
	if err := json.Unmarshal(body, &request); err != nil {
		log.Printf("Error parsing body %v", err)
		http.Error(w, "Error parsing body", http.StatusBadRequest)
	}

	result, err := checkRequest(request.Request, handler)
	response := admission.AdmissionResponse{
		UID: request.Request.UID,
		Allowed: result,
	}
	if err != nil {
		response.Result = &k8meta.Status{
			Message: fmt.Sprintf("%v", err),
			Reason: k8meta.StatusReasonUnauthorized,
		}
	}

	json, err := json.Marshal(response)
	log.Printf("result: %+v, %v", string(json), err)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error encoding response %v", err), http.StatusInternalServerError)
	} else {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(json); err != nil {
			log.Printf("Error writing response %v", err)
			http.Error(w, fmt.Sprintf("Error writing response: %v", err), http.StatusInternalServerError)
		}
	}
}

func checkRequest(request *admission.AdmissionRequest, handler *AdmissionHandler) (bool, error) {
	return true, nil
	if request.Namespace == "kube-system" {
		fmt.Printf("Warning: Controller is applied to kube-system, skipping")
		return true, nil
	}

	if request.RequestKind.Group != "batch" || request.RequestKind.Kind != "Job" || request.Operation != "CREATE" {
		log.Printf("Skipped resource [%v,%v,%v], check rules to exclude this resource", request.RequestKind.Group, request.RequestKind.Kind, request.Operation)
		return true, nil
	}

	var job *batchv1.Job
	err := json.Unmarshal(request.Object.Raw, &job)
	if err != nil {
		log.Printf("Error parsing job %v", err)
		return true, nil
	}

	return checkJob(job, handler)
}

/// Check that the given job has the runtimeclass and exclude denied parameters
/// We must check:
/// - RuntimeClass is the correct value
/// - SecurityContext.RunAsNonRoot must be set
/// - SecurityContext.AllowPrivilegeEscalation
/// - The container has no volumes (Except secrets)
/// - Network is not hostnetwork
/// - Container ports is empty
func checkJob(request *batchv1.Job, handler *AdmissionHandler) (bool, error) {
	log.Printf("Checking Job: %+v", request.Spec.Template.Spec)
	spec := request.Spec.Template.Spec
	if spec.RuntimeClassName != nil && *spec.RuntimeClassName != handler.RuntimeClass {
		return false, fmt.Errorf("wrong RuntimeClass %v is set for job %v", spec.RuntimeClassName, request.Name)
	}

	if spec.HostNetwork != false {
		return false, fmt.Errorf("HostNetwork must not be set")
	}

	if spec.HostIPC != false {
		return false, fmt.Errorf("HostIPC must be false")
	}

	if spec.HostPID != false {
		return false, fmt.Errorf("HostPID must be false")
	}

	for _, container := range spec.Containers {
		if container.SecurityContext == nil {
			return false, fmt.Errorf("SecurityContext must be set for the container")
		}
		context := *container.SecurityContext

		if context.RunAsNonRoot != nil && *context.RunAsNonRoot != true {
			return false, fmt.Errorf("RunAsNonRoot must be set per container")
		}

		if context.AllowPrivilegeEscalation != nil && *context.AllowPrivilegeEscalation != false {
			return false, fmt.Errorf("AllowPrivilegeEscalation must be false per container")
		}

		if context.Privileged != nil && *context.Privileged != false {
			return false, fmt.Errorf("Privileged must be false per container")
		}

		if len(context.Capabilities.Drop) != 1 || context.Capabilities.Drop[0] != "all" {
			return false, fmt.Errorf("Container must drop all capabilities (Only 'all' must be set)")
		}

		if len(container.Ports) > 0 {
			return false, fmt.Errorf("No port must be defined")
		}
	}

	for _, volume := range spec.Volumes {
		// You can only mount secrets (ServiceAccount from current namespace)
		if volume.Secret == nil {
			return false, fmt.Errorf("No volumes are allowed")
		}
	}

	return true, nil
}
