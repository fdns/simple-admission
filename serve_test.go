package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	admission "k8s.io/api/admission/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
)

func loadValidJob(t *testing.T) admission.AdmissionReview {
	body := `{"kind":"AdmissionReview","apiVersion":"admission.k8s.io/v1","request":{"uid":"bb1d3b77-a678-4aec-9dff-846120cf1675","kind":{"group":"batch","version":"v1","kind":"Job"},"resource":{"group":"batch","version":"v1","resource":"jobs"},"requestKind":{"group":"batch","version":"v1","kind":"Job"},"requestResource":{"group":"batch","version":"v1","resource":"jobs"},"name":"test","namespace":"default","operation":"CREATE","userInfo":{"username":"kubernetes-admin","groups":["system:masters","system:authenticated"]},"object":{"kind":"Job","apiVersion":"batch/v1","metadata":{"name":"test","namespace":"default","uid":"0ad3e67c-42aa-4e5f-bf31-e58dd8258b74","creationTimestamp":"2021-03-23T01:14:52Z","annotations":{"kubectl.kubernetes.io/last-applied-configuration":"{\"apiVersion\":\"batch/v1\",\"kind\":\"Job\",\"metadata\":{\"annotations\":{},\"creationTimestamp\":null,\"name\":\"test\",\"namespace\":\"default\"},\"spec\":{\"template\":{\"metadata\":{\"creationTimestamp\":null},\"spec\":{\"containers\":[{\"command\":[\"echo\",\"1\"],\"image\":\"nginx\",\"name\":\"test\",\"resources\":{},\"securityContext\":{\"allowPrivilegeEscalation\":false,\"capabilities\":{\"drop\":[\"all\"]},\"privileged\":false,\"runAsNonRoot\":true}}],\"restartPolicy\":\"Never\",\"runtimeClassName\":\"gvisor\"}}},\"status\":{}}\n"},"managedFields":[{"manager":"kubectl-client-side-apply","operation":"Update","apiVersion":"batch/v1","time":"2021-03-23T01:14:52Z","fieldsType":"FieldsV1","fieldsV1":{"f:metadata":{"f:annotations":{".":{},"f:kubectl.kubernetes.io/last-applied-configuration":{}}},"f:spec":{"f:backoffLimit":{},"f:completions":{},"f:parallelism":{},"f:template":{"f:spec":{"f:containers":{"k:{\"name\":\"test\"}":{".":{},"f:command":{},"f:image":{},"f:imagePullPolicy":{},"f:name":{},"f:resources":{},"f:securityContext":{".":{},"f:allowPrivilegeEscalation":{},"f:capabilities":{".":{},"f:drop":{}},"f:privileged":{},"f:runAsNonRoot":{}},"f:terminationMessagePath":{},"f:terminationMessagePolicy":{}}},"f:dnsPolicy":{},"f:restartPolicy":{},"f:runtimeClassName":{},"f:schedulerName":{},"f:securityContext":{},"f:terminationGracePeriodSeconds":{}}}}}}]},"spec":{"parallelism":1,"completions":1,"backoffLimit":6,"selector":{"matchLabels":{"controller-uid":"0ad3e67c-42aa-4e5f-bf31-e58dd8258b74"}},"template":{"metadata":{"creationTimestamp":null,"labels":{"controller-uid":"0ad3e67c-42aa-4e5f-bf31-e58dd8258b74","job-name":"test"}},"spec":{"containers":[{"name":"test","image":"nginx","command":["echo","1"],"resources":{},"terminationMessagePath":"/dev/termination-log","terminationMessagePolicy":"File","imagePullPolicy":"Always","securityContext":{"capabilities":{"drop":["all"]},"privileged":false,"runAsNonRoot":true,"allowPrivilegeEscalation":false}}],"restartPolicy":"Never","terminationGracePeriodSeconds":30,"dnsPolicy":"ClusterFirst","securityContext":{},"schedulerName":"default-scheduler","runtimeClassName":"gvisor"}}},"status":{}},"oldObject":null,"dryRun":false,"options":{"kind":"CreateOptions","apiVersion":"meta.k8s.io/v1","fieldManager":"kubectl-client-side-apply"}}}`
	adm := admission.AdmissionReview{}
	if err := json.Unmarshal([]byte(body), &adm); err != nil {
		t.Fatalf("Error loading job %v", err)
	}
	return adm
}

func loadJob(t *testing.T, request admission.AdmissionReview) *batchv1.Job {
	var job *batchv1.Job
	err := json.Unmarshal(request.Request.Object.Raw, &job)
	if err != nil {
		t.Fatalf("Error parsing job %v", err)
	}
	return job
}

func saveJob(t *testing.T, request admission.AdmissionReview, job *batchv1.Job) {
	result, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("Error parsing job %v", err)
	}
	request.Request.Object.Raw = result
}

func sendRequest(t *testing.T, job admission.AdmissionReview) admission.AdmissionReview {
	encoded, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("Error loading json %v", err)
	}
	req, err := http.NewRequest("POST", "/health-check", bytes.NewReader([]byte(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler := AdmissionHandler{
		RuntimeClass: "gvisor",
	}

	handler.handler(rr, req)
	if rr.Code != 200 {
		t.Fatalf("Handler returned wrong status code, expected 200, got %v", rr.Code)
	}

	// Check body response
	body, err := ioutil.ReadAll(rr.Body)
	response := admission.AdmissionReview{}
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	return response
}

func TestValidJob(t *testing.T) {
	admission := loadValidJob(t)
	job := loadJob(t, admission)
	saveJob(t, admission, job)

	response := sendRequest(t, admission)
	if response.Response.Allowed == false {
		t.Fatalf("Error validating valid job, %v", response.Response.Result.Message)
	}
}

func TestEnvVariable(t *testing.T) {
	admission := loadValidJob(t)
	job := loadJob(t, admission)
	job.Spec.Template.Spec.Containers[0].Env = []v1.EnvVar{v1.EnvVar{
		Name:  "ValidEnv",
		Value: "Value",
	}}
	saveJob(t, admission, job)

	response := sendRequest(t, admission)
	if response.Response.Allowed == false {
		t.Fatalf("Error validating valid job, %v", response.Response.Result.Message)
	}
}

func TestInvalidMinimalJob(t *testing.T) {
	job := `{"kind":"AdmissionReview","apiVersion":"admission.k8s.io/v1","request":{"uid":"33ac6f03-52a9-4df5-9739-5bb814227002","kind":{"group":"batch","version":"v1","kind":"Job"},"resource":{"group":"batch","version":"v1","resource":"jobs"},"requestKind":{"group":"batch","version":"v1","kind":"Job"},"requestResource":{"group":"batch","version":"v1","resource":"jobs"},"name":"test","namespace":"default","operation":"CREATE","userInfo":{"username":"kubernetes-admin","groups":["system:masters","system:authenticated"]},"object":{"kind":"Job","apiVersion":"batch/v1","metadata":{"name":"test","namespace":"default","uid":"39b0000f-962f-4d9b-adeb-d47dd332e902","creationTimestamp":"2021-03-22T23:41:46Z","managedFields":[{"manager":"kubectl-create","operation":"Update","apiVersion":"batch/v1","time":"2021-03-22T23:41:46Z","fieldsType":"FieldsV1","fieldsV1":{"f:spec":{"f:backoffLimit":{},"f:completions":{},"f:parallelism":{},"f:template":{"f:spec":{"f:containers":{"k:{\"name\":\"test\"}":{".":{},"f:command":{},"f:image":{},"f:imagePullPolicy":{},"f:name":{},"f:resources":{},"f:terminationMessagePath":{},"f:terminationMessagePolicy":{}}},"f:dnsPolicy":{},"f:restartPolicy":{},"f:schedulerName":{},"f:securityContext":{},"f:terminationGracePeriodSeconds":{}}}}}}]},"spec":{"parallelism":1,"completions":1,"backoffLimit":6,"selector":{"matchLabels":{"controller-uid":"39b0000f-962f-4d9b-adeb-d47dd332e902"}},"template":{"metadata":{"creationTimestamp":null,"labels":{"controller-uid":"39b0000f-962f-4d9b-adeb-d47dd332e902","job-name":"test"}},"spec":{"containers":[{"name":"test","image":"nginx","command":["echo","1"],"resources":{},"terminationMessagePath":"/dev/termination-log","terminationMessagePolicy":"File","imagePullPolicy":"Always"}],"restartPolicy":"Never","terminationGracePeriodSeconds":30,"dnsPolicy":"ClusterFirst","securityContext":{},"schedulerName":"default-scheduler"}}},"status":{}},"oldObject":null,"dryRun":false,"options":{"kind":"CreateOptions","apiVersion":"meta.k8s.io/v1","fieldManager":"kubectl-create"}}}`
	adm := admission.AdmissionReview{}
	if err := json.Unmarshal([]byte(job), &adm); err != nil {
		t.Fatalf("Error loading job %v", err)
	}
	response := sendRequest(t, adm)
	if response.Response.Allowed == true {
		t.Fatalf("Invalid job returned as valid")
	}
}

func TestInvalidCases(t *testing.T) {
	errorMap := map[string]interface{}{
		"nosecuritypolicy": func(job *batchv1.Job) { job.Spec.Template.Spec.Containers[0].SecurityContext = nil },
		"nocaps":           func(job *batchv1.Job) { job.Spec.Template.Spec.Containers[0].SecurityContext.Capabilities = nil },
		"addcaps": func(job *batchv1.Job) {
			job.Spec.Template.Spec.Containers[0].SecurityContext.Capabilities.Add = []v1.Capability{"NET_ADMIN"}
		},
		"addcapsnodrop": func(job *batchv1.Job) {
			job.Spec.Template.Spec.Containers[0].SecurityContext.Capabilities.Drop = []v1.Capability{}
			job.Spec.Template.Spec.Containers[0].SecurityContext.Capabilities.Add = []v1.Capability{"NET_ADMIN"}
		},
		"allowpriv": func(job *batchv1.Job) {
			*job.Spec.Template.Spec.Containers[0].SecurityContext.AllowPrivilegeEscalation = true
		},
		"nonrootnotset": func(job *batchv1.Job) {
			job.Spec.Template.Spec.Containers[0].SecurityContext.RunAsNonRoot = nil
		},
		"nonroot": func(job *batchv1.Job) {
			*job.Spec.Template.Spec.Containers[0].SecurityContext.RunAsNonRoot = false
		},
		"privileged": func(job *batchv1.Job) {
			*job.Spec.Template.Spec.Containers[0].SecurityContext.Privileged = true
		},
		"sysctl": func(job *batchv1.Job) {
			job.Spec.Template.Spec.SecurityContext.Sysctls = []v1.Sysctl{v1.Sysctl{}}
		},
		"hostnet": func(job *batchv1.Job) {
			job.Spec.Template.Spec.HostNetwork = true
		},
		"hostpid": func(job *batchv1.Job) {
			job.Spec.Template.Spec.HostPID = true
		},
		"hostipc": func(job *batchv1.Job) {
			job.Spec.Template.Spec.HostIPC = true
		},
		"servaccount": func(job *batchv1.Job) {
			job.Spec.Template.Spec.ServiceAccountName = "test"
		},
		"restartpolicy": func(job *batchv1.Job) {
			job.Spec.Template.Spec.RestartPolicy = "Always"
		},
		"runtimeclassnil": func(job *batchv1.Job) {
			job.Spec.Template.Spec.RuntimeClassName = nil
		},
		"runtimeclass": func(job *batchv1.Job) {
			*job.Spec.Template.Spec.RuntimeClassName = "default"
		},
		"ports": func(job *batchv1.Job) {
			job.Spec.Template.Spec.Containers[0].Ports = []v1.ContainerPort{v1.ContainerPort{}}
		},
		"invalidEnv": func(job *batchv1.Job) {
			job.Spec.Template.Spec.Containers[0].Env = []v1.EnvVar{v1.EnvVar{
				Name:      "Invalid",
				ValueFrom: &v1.EnvVarSource{},
			}}
		},
		"envFrom": func(job *batchv1.Job) {
			job.Spec.Template.Spec.Containers[0].EnvFrom = []v1.EnvFromSource{v1.EnvFromSource{}}
		},
		"volumeDevice": func(job *batchv1.Job) {
			job.Spec.Template.Spec.Containers[0].VolumeDevices = []v1.VolumeDevice{v1.VolumeDevice{}}
		},
		"volumeMount": func(job *batchv1.Job) {
			job.Spec.Template.Spec.Containers[0].VolumeMounts = []v1.VolumeMount{v1.VolumeMount{}}
		},
		"volume": func(job *batchv1.Job) {
			job.Spec.Template.Spec.Volumes = []v1.Volume{v1.Volume{}}
		},
	}

	for key, val := range errorMap {
		admission := loadValidJob(t)
		job := loadJob(t, admission)
		val.(func(*batchv1.Job))(job)
		saveJob(t, admission, job)

		response := sendRequest(t, admission)
		if response.Response.Allowed == true {
			t.Fatalf("Invalid job `%v` was allowed", key)
		}
	}
}

func TestInvalidJson(t *testing.T) {
	req, err := http.NewRequest("POST", "/health-check", bytes.NewReader([]byte("invalidjson")))
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler := AdmissionHandler{
		RuntimeClass: "TestRuntimeClass",
	}

	handler.handler(rr, req)
	if rr.Code != 400 {
		t.Errorf("Handler returned wrong status code, expected 400, got %v", rr.Code)
	}
}

func TestEmptyBody(t *testing.T) {
	req, err := http.NewRequest("POST", "/health-check", bytes.NewReader([]byte("")))
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler := AdmissionHandler{
		RuntimeClass: "TestRuntimeClass",
	}

	handler.handler(rr, req)
	if rr.Code != 400 {
		t.Errorf("Handler returned wrong status code, expected 400, got %v", rr.Code)
	}
}

func TestNonBatchKind(t *testing.T) {
	admission := loadValidJob(t)
	admission.Request.Kind.Kind = "Invalid"

	response := sendRequest(t, admission)
	if response.Response.Allowed == false {
		t.Fatalf("Invalid admission kind was processed, %v", response.Response.Result.Message)
	}
}

func TestNonBatchGroup(t *testing.T) {
	admission := loadValidJob(t)
	admission.Request.Kind.Group = "Invalid"

	response := sendRequest(t, admission)
	if response.Response.Allowed == false {
		t.Fatalf("Invalid admission kind was processed, %v", response.Response.Result.Message)
	}
}

func TestNonBatchOperation(t *testing.T) {
	admission := loadValidJob(t)
	admission.Request.Operation = "PATCH"

	response := sendRequest(t, admission)
	if response.Response.Allowed == false {
		t.Fatalf("Invalid admission kind was processed, %v", response.Response.Result.Message)
	}
}
