# Copilot Review Prow Plugin

A [Prow external plugin](https://docs.prow.k8s.io/docs/components/plugins/external-plugins/)
that lets GitHub org members request a
[GitHub Copilot code review](https://docs.github.com/en/copilot/using-github-copilot/code-review/using-copilot-code-review)
on a pull request by commenting `/copilot-review`.

It uses a shared org-level token so that users without their own Copilot
seat can still trigger a review.

## Usage

Comment on any pull request in a configured repository:

```text
/copilot-review
```

The plugin will verify that the commenter is a member of the GitHub org,
then add `@copilot` as a reviewer using the `gh` CLI.

## Environment Variables

| Variable | Description |
|----------|-------------|
| `COPILOT_REVIEW_TOKEN` | GitHub token used by `gh` CLI to request the review (highest priority) |
| `GH_TOKEN` | Fallback GitHub token if `COPILOT_REVIEW_TOKEN` is not set |
| `GITHUB_TOKEN` | Fallback if neither of the above is set |

At least one of these must be set to a token with permission to add
reviewers to pull requests.

## Command-Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `8888` | Port to listen on |
| `--dry-run` | `true` | Dry run mode — uses API tokens but does not mutate |
| `--hmac-secret-file` | `/etc/webhook/hmac` | Path to the GitHub HMAC webhook secret |

Standard Prow GitHub and instrumentation flags are also supported.

## Building the Image

```bash
docker build -t copilot-review-prow-plugin copilot-review-prow-plugin/
```

The Dockerfile compiles the Go binary and bundles the
[`gh` CLI](https://cli.github.com/) into a distroless image.

## End-to-end test with a real PR (with real containers)

This runs the plugin in a Kind cluster and sends a webhook to trigger a real
Copilot review request.

**Prerequisites:**
- Docker, kind, and kubectl installed
- `GH_TOKEN` set to a token with repo + Copilot access
- A second token for the Prow GitHub client (comments/reactions), or reuse the same one

```bash
# 1. Create a Kind cluster
kind create cluster --name copilot-review-test

# 2. Build the Docker image
# cd setup/prow-plugin/plugin
docker build -t copilot-review:latest .

# 3. Load the image into Kind
kind load docker-image copilot-review:latest --name copilot-review-test

# 4. Create the namespace and secrets
kubectl create namespace prow

kubectl create secret generic hmac-token \
  --namespace prow \
  --from-literal=hmac=test-secret

# Verify token is set: echo $GH_TOKEN
kubectl create secret generic copilot-review-github-token \
  --namespace prow \
  --from-literal=token="$GH_TOKEN"

kubectl create secret generic copilot-review-bot-github-token \
  --namespace prow \
  --from-literal=token="$GH_TOKEN"

# 5. Apply the deployment (patch the image since we loaded locally)
kubectl apply -f deployment/ -n prow
kubectl set image deployment/copilot-review \
  copilot-review=copilot-review:latest -n prow
kubectl patch deployment copilot-review -n prow \
  -p '{"spec":{"template":{"spec":{"containers":[{"name":"copilot-review","imagePullPolicy":"IfNotPresent"}]}}}}'

# 6. Wait for the pod to be ready
kubectl rollout status deployment/copilot-review -n prow --timeout=60s

# 7. Port-forward to the plugin service
kubectl port-forward -n prow svc/copilot-review 8888:80 &
PF_PID=$!
sleep 2

# 8. Send a test webhook
cat <<'EOF' > /tmp/payload.json
{
  "action": "created",
  "issue": {
    "number": 3,
    "pull_request": {"url": "https://api.github.com/repos/peppi-lotta/playground/pulls/3"},
    "state": "open"
  },
  "comment": {
    "id": 1,
    "body": "/copilot-review",
    "user": {"login": "peppi-lotta"}
  },
  "repository": {
    "name": "playground",
    "owner": {"login": "peppi-lotta"},
    "full_name": "peppi-lotta/playground"
  }
}
EOF

SIGNATURE=$(cat /tmp/payload.json | openssl dgst -sha256 -hmac "test-secret" | awk '{print "sha256="$2}')

curl -X POST http://localhost:8888 \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: issue_comment" \
  -H "X-GitHub-Delivery: test-$(date +%s)" \
  -H "X-Hub-Signature-256: $SIGNATURE" \
  --data-binary @/tmp/payload.json

# 9. Check the logs
kubectl logs -n prow -l app=copilot-review --tail=50

# 10. Clean up
kill $PF_PID
kind delete cluster --name copilot-review-test
```
### DEBUG

```bash
# Run gh inside the container
kubectl exec -n prow deploy/copilot-review -- /usr/bin/gh version

# Run the main binary with --help
kubectl exec -n prow deploy/copilot-review -- /copilot-review --help

# For an interactive shell, use a debug container that shares the pod's process namespace:
kubectl debug -n prow -it $(kubectl get pod -n prow -l app=copilot-review -o jsonpath='{.items[0].metadata.name}') \
  --image=alpine:3.23 --target=copilot-review -- /bin/sh
```