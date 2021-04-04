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
	"k8s.io/apimachinery/pkg/api/resource"
)

func loadValidJob(t *testing.T) admission.AdmissionReview {
	body := `{"kind":"AdmissionReview","apiVersion":"admission.k8s.io/v1","request":{"uid":"e1b0cabd-700d-4ed2-90ab-7e0957da7037","kind":{"group":"batch","version":"v1","kind":"Job"},"resource":{"group":"batch","version":"v1","resource":"jobs"},"requestKind":{"group":"batch","version":"v1","kind":"Job"},"requestResource":{"group":"batch","version":"v1","resource":"jobs"},"name":"busybox","namespace":"default","operation":"CREATE","userInfo":{"username":"kubernetes-admin","groups":["system:masters","system:authenticated"]},"object":{"kind":"Job","apiVersion":"batch/v1","metadata":{"name":"busybox","namespace":"default","uid":"c0381bc6-f7dd-49b3-a21d-4aa95f0f2d7b","creationTimestamp":"2021-04-04T22:30:26Z","annotations":{"kubectl.kubernetes.io/last-applied-configuration":"{\"apiVersion\":\"batch/v1\",\"kind\":\"Job\",\"metadata\":{\"annotations\":{},\"name\":\"busybox\",\"namespace\":\"default\"},\"spec\":{\"activeDeadlineSeconds\":30,\"backoffLimit\":1,\"template\":{\"spec\":{\"containers\":[{\"command\":[\"sleep\",\"120\"],\"env\":[{\"name\":\"TEST\",\"value\":\"VALUE\"}],\"image\":\"busybox\",\"name\":\"busybox\",\"resources\":{\"limits\":{\"cpu\":\"10m\",\"memory\":\"50Mi\"},\"requests\":{\"cpu\":\"10m\",\"memory\":\"50Mi\"}},\"securityContext\":{\"allowPrivilegeEscalation\":false,\"capabilities\":{\"drop\":[\"all\"]},\"privileged\":false,\"runAsNonRoot\":true,\"runAsUser\":33}}],\"restartPolicy\":\"Never\",\"runtimeClassName\":\"gvisor\"}},\"ttlSecondsAfterFinished\":86400}}\n"},"managedFields":[{"manager":"kubectl-client-side-apply","operation":"Update","apiVersion":"batch/v1","time":"2021-04-04T22:30:26Z","fieldsType":"FieldsV1","fieldsV1":{"f:metadata":{"f:annotations":{".":{},"f:kubectl.kubernetes.io/last-applied-configuration":{}}},"f:spec":{"f:activeDeadlineSeconds":{},"f:backoffLimit":{},"f:completions":{},"f:parallelism":{},"f:template":{"f:spec":{"f:containers":{"k:{\"name\":\"busybox\"}":{".":{},"f:command":{},"f:env":{".":{},"k:{\"name\":\"TEST\"}":{".":{},"f:name":{},"f:value":{}}},"f:image":{},"f:imagePullPolicy":{},"f:name":{},"f:resources":{".":{},"f:limits":{".":{},"f:cpu":{},"f:memory":{}},"f:requests":{".":{},"f:cpu":{},"f:memory":{}}},"f:securityContext":{".":{},"f:allowPrivilegeEscalation":{},"f:capabilities":{".":{},"f:drop":{}},"f:privileged":{},"f:runAsNonRoot":{},"f:runAsUser":{}},"f:terminationMessagePath":{},"f:terminationMessagePolicy":{}}},"f:dnsPolicy":{},"f:restartPolicy":{},"f:runtimeClassName":{},"f:schedulerName":{},"f:securityContext":{},"f:terminationGracePeriodSeconds":{}}},"f:ttlSecondsAfterFinished":{}}}}]},"spec":{"parallelism":1,"completions":1,"activeDeadlineSeconds":30,"backoffLimit":1,"selector":{"matchLabels":{"controller-uid":"c0381bc6-f7dd-49b3-a21d-4aa95f0f2d7b"}},"template":{"metadata":{"creationTimestamp":null,"labels":{"controller-uid":"c0381bc6-f7dd-49b3-a21d-4aa95f0f2d7b","job-name":"busybox"}},"spec":{"containers":[{"name":"busybox","image":"busybox","command":["sleep","120"],"env":[{"name":"TEST","value":"VALUE"}],"resources":{"limits":{"cpu":"10m","memory":"50Mi"},"requests":{"cpu":"10m","memory":"50Mi"}},"terminationMessagePath":"/dev/termination-log","terminationMessagePolicy":"File","imagePullPolicy":"Always","securityContext":{"capabilities":{"drop":["all"]},"privileged":false,"runAsUser":33,"runAsNonRoot":true,"allowPrivilegeEscalation":false}}],"restartPolicy":"Never","terminationGracePeriodSeconds":30,"dnsPolicy":"ClusterFirst","securityContext":{},"schedulerName":"default-scheduler","runtimeClassName":"gvisor"}}},"status":{}},"oldObject":null,"dryRun":false,"options":{"kind":"CreateOptions","apiVersion":"meta.k8s.io/v1","fieldManager":"kubectl-client-side-apply"}}}`
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
		"noactiveDeadlineSeconds":   func(job *batchv1.Job) { job.Spec.ActiveDeadlineSeconds = nil },
		"zeroactiveDeadlineSeconds": func(job *batchv1.Job) { *job.Spec.ActiveDeadlineSeconds = 0 },
		"nobackoff":                 func(job *batchv1.Job) { job.Spec.BackoffLimit = nil },
		"notonebackoff":             func(job *batchv1.Job) { *job.Spec.BackoffLimit = 2 },
		"parallelism":               func(job *batchv1.Job) { *job.Spec.Parallelism = 2 },
		"completions":               func(job *batchv1.Job) { *job.Spec.Completions = 2 },
		"nosecuritypolicy":          func(job *batchv1.Job) { job.Spec.Template.Spec.Containers[0].SecurityContext = nil },
		"nocaps":                    func(job *batchv1.Job) { job.Spec.Template.Spec.Containers[0].SecurityContext.Capabilities = nil },
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
		"nocpu": func(job *batchv1.Job) {
			job.Spec.Template.Spec.Containers[0].Resources.Requests[v1.ResourceCPU] = resource.MustParse("0")
		},
		"nocpuequal": func(job *batchv1.Job) {
			job.Spec.Template.Spec.Containers[0].Resources.Requests[v1.ResourceCPU] = resource.MustParse("30m")
		},
		"nomem": func(job *batchv1.Job) {
			job.Spec.Template.Spec.Containers[0].Resources.Requests[v1.ResourceMemory] = resource.MustParse("0")
		},
		"nomemequal": func(job *batchv1.Job) {
			job.Spec.Template.Spec.Containers[0].Resources.Requests[v1.ResourceMemory] = resource.MustParse("30Mi")
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
