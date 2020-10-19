# template-autoops-cronjob

## Usage

Create namespace `autoops` and apply yaml resources as described below.

```yaml
# create serviceaccount
apiVersion: v1
kind: ServiceAccount
metadata:
  name: template-autoops-cronjob
  namespace: autoops
---
# create clusterrole
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: template-autoops-cronjob
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["list"]
---
# create clusterrolebinding
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: template-autoops-cronjob
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: template-autoops-cronjob
subjects:
  - kind: ServiceAccount
    name: template-autoops-cronjob
    namespace: autoops
---
# create cronjob
apiVersion: batch/v1beta1
kind: CronJob
metadata:
  name: template-autoops-cronjob
  namespace: autoops
spec:
  schedule: "*/5 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          serviceAccount: template-autoops-cronjob
          containers:
            - name: template-autoops-cronjob
              image: autoops/template-autoops-cronjob
          restartPolicy: OnFailure
```

## Credits

Guo Y.K., MIT License
