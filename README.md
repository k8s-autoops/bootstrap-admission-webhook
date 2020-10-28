# admission-bootstrapper

A tool to deploy admission webhooks in kubernetes, no need to concern any certificate signing issues.

## Usage

### Initialization

* Create namespace `autoops`
* Apply resources as described below

```yaml
# create serviceaccount
apiVersion: v1
kind: ServiceAccount
metadata:
  name: admission-bootstrapper
  namespace: autoops
---
# create clusterrole
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: admission-bootstrapper
rules:
  - apiGroups: [""]
    resources: ["secrets", "services"]
    verbs: ["get", "create"]
  - apiGroups: ["apps"]
    resources: ["statefulsets"]
    verbs: ["get", "create"]
  - apiGroups: ["admissionregistration.k8s.io"]
    resources: ["mutatingwebhookconfigurations", "validatingwebhookconfigurations"]
    verbs: ["get", "create"]
---
# create clusterrolebinding
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: admission-bootstrapper
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: admission-bootstrapper
subjects:
  - kind: ServiceAccount
    name: admission-bootstrapper
    namespace: autoops
```

### Deploy an Admission Webhook

**Image Requirements**

* Listens at `:443`
* Use certificates from `/autoops-data/tls/tls.crt` and `/autoops-data/tls/tls.key`

**Example**

See https://github.com/k8s-autoops/admission-httpscat

Here is an example to deploy `admission-httpscat` to validate any `CREATE` operations of `ConfigMap`

```yaml
# create job
apiVersion: batch/v1
kind: Job
metadata:
  # !!!CHANGE ME!!!
  name: admission-bootstrapper-httpscat
  namespace: autoops
spec:
  template:
    spec:
      serviceAccount: admission-bootstrapper
      containers:
        - name: admission-bootstrapper
          image: autoops/admission-bootstrapper
          env:
            - name: ADMISSION_NAME
              value: admission-httpscat
            - name: ADMISSION_IMAGE
              value: autoops/admission-httpscat
            - name: ADMISSION_ENVS
              value: "AAAA=BBBB;CCCC=DDDD"
            - name: ADMISSION_MUTATING
              value: "false"
            - name: ADMISSION_IGNORE_FAILURE
              value: "true"
            - name: ADMISSION_SIDE_EFFECT
              value: "None"
            - name: ADMISSION_RULES
              value: '[{"operations":["CREATE"],"apiGroups":[""], "apiVersions":["*"], "resources":["configmaps"]}]'
      restartPolicy: OnFailure
```


## Credits

Guo Y.K., MIT License
