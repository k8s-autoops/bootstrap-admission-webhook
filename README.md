# admission-bootstrapper

## Usage

Create namespace `autoops` and apply yaml resources as described below.

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
    resources: ["secrets"]
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

Here is a sample to install a admission webhoook, check their own `README`.

```yaml
# create job
apiVersion: batch/v1
kind: Job
metadata:
  # !!!CHANGE ME!!!
  name: admission-bootstrapper-xxxxx
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
              # !!!CHANGE ME!!!
              value: xxxxx-xxxxx
            - name: ADMISSION_IMAGE
              # !!!CHANGE ME!!!
              value: autoops/auto-xxxxxx-xxxxxx
            - name: ADMISSION_CFG
              # !!!CHANGE ME!!!
              value: xxxxxxxx
      restartPolicy: OnFailure
```

## Credits

Guo Y.K., MIT License
