package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	admission "k8s.io/api/admission/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	k8meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AdmissionHandler struct {
	RuntimeClass string
}

// Handle requests
func (handler *AdmissionHandler) handler(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		data, err := ioutil.ReadAll(r.Body)
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

	request := admission.AdmissionReview{}
	if err := json.Unmarshal(body, &request); err != nil {
		log.Printf("Error parsing body %v", err)
		http.Error(w, "Error parsing body", http.StatusBadRequest)
		return
	}

	result, err := checkRequest(request.Request, handler)
	response := admission.AdmissionResponse{
		UID:     request.Request.UID,
		Allowed: result,
	}
	if err != nil {
		response.Result = &k8meta.Status{
			Message: fmt.Sprintf("%v", err),
			Reason:  k8meta.StatusReasonUnauthorized,
		}
	}

	outReview := admission.AdmissionReview{
		TypeMeta: request.TypeMeta,
		Request:  request.Request,
		Response: &response,
	}
	json, err := json.Marshal(outReview)

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
	if request.Namespace == "kube-system" {
		log.Printf("Warning: Controller is applied to kube-system, skipping")
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

// Check that the applied job has all the security properties set
func checkJob(request *batchv1.Job, handler *AdmissionHandler) (bool, error) {
	if request.Spec.ActiveDeadlineSeconds == nil || *request.Spec.ActiveDeadlineSeconds == 0 {
		return false, fmt.Errorf("activeDeadlineSeconds must be set")
	}

	if request.Spec.BackoffLimit == nil || *request.Spec.BackoffLimit != 1 {
		return false, fmt.Errorf("backoffLimit mus be set to 1")
	}

	if request.Spec.Parallelism != nil && *request.Spec.Parallelism != 1 {
		return false, fmt.Errorf("Parallelism must not be used")
	}

	if request.Spec.Completions != nil && *request.Spec.Completions != 1 {
		return false, fmt.Errorf("Completions must not be used")
	}

	// TTLSecondsAfterFinished is an alpha feature, and must be enabled manually
	//if request.Spec.TTLSecondsAfterFinished == nil || *request.Spec.TTLSecondsAfterFinished == 0 {
	//	return false, fmt.Errorf("ttlSecondsAfterFinished must be set greater than 0")
	//}

	spec := request.Spec.Template.Spec
	if spec.RuntimeClassName == nil || *spec.RuntimeClassName != handler.RuntimeClass {
		return false, fmt.Errorf("wrong RuntimeClass %v is set for job %v, must be %v", spec.RuntimeClassName, request.Name, handler.RuntimeClass)
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

	if spec.ServiceAccountName != "" {
		return false, fmt.Errorf("You must not set a serviceAccount")
	}

	if spec.RestartPolicy != "Never" {
		return false, fmt.Errorf("Job is not allowed to restart")
	}

	if spec.SecurityContext != nil && len(spec.SecurityContext.Sysctls) > 0 {
		return false, fmt.Errorf("Sysctls must be empty")
	}

	for _, container := range spec.Containers {
		if container.SecurityContext == nil {
			return false, fmt.Errorf("SecurityContext must be set for the container")
		}
		context := *container.SecurityContext

		if context.RunAsNonRoot == nil || *context.RunAsNonRoot != true {
			return false, fmt.Errorf("RunAsNonRoot must be set per container")
		}

		if context.AllowPrivilegeEscalation == nil || *context.AllowPrivilegeEscalation != false {
			return false, fmt.Errorf("AllowPrivilegeEscalation must be false per container")
		}

		if context.Privileged == nil || *context.Privileged != false {
			return false, fmt.Errorf("Privileged must be false per container")
		}

		if context.Capabilities == nil || len(context.Capabilities.Drop) != 1 || context.Capabilities.Drop[0] != "all" {
			return false, fmt.Errorf("Container must drop all capabilities (Only 'all' must be set)")
		}

		if len(context.Capabilities.Add) > 0 {
			return false, fmt.Errorf("Container must not add any capabilites")
		}

		if len(container.Ports) > 0 {
			return false, fmt.Errorf("No port must be defined")
		}

		if len(container.EnvFrom) > 0 {
			return false, fmt.Errorf("EnvFrom must not be defined")
		}

		for _, env := range container.Env {
			if env.ValueFrom != nil {
				return false, fmt.Errorf("env valueFrom can't be defined")
			}
		}

		if len(container.VolumeDevices) > 0 {
			return false, fmt.Errorf("VolumeDevices are not supported")
		}

		if len(container.VolumeMounts) > 0 {
			return false, fmt.Errorf("VolumeMounts are not supported")
		}

		if container.Resources.Requests.Cpu() == nil || container.Resources.Limits.Cpu() == nil || container.Resources.Requests.Cpu().IsZero() || container.Resources.Limits.Cpu().IsZero() {
			return false, fmt.Errorf("Container cpu requests and limit must be set")
		}

		if !container.Resources.Requests.Cpu().Equal(*container.Resources.Limits.Cpu()) {
			return false, fmt.Errorf("CPU request must be set and equal to limits")
		}

		if container.Resources.Requests.Memory() == nil || container.Resources.Limits.Memory() == nil || container.Resources.Requests.Memory().IsZero() || container.Resources.Limits.Memory().IsZero() {
			return false, fmt.Errorf("Container memory requests and limit must be set")
		}

		if !container.Resources.Requests.Memory().Equal(*container.Resources.Limits.Memory()) {
			return false, fmt.Errorf("Memory request must be set and equal to limits")
		}
	}

	if len(spec.Volumes) > 0 {
		return false, fmt.Errorf("There are more than one volume declared %v", len(spec.Volumes))
	}

	return true, nil
}
