# Remote Image Retag (`docker-retag`)

`docker-retag` is a lightweight, single-purpose CLI tool and GitHub Action to point a "floating tag" (like `:production` or `:staging`) to a new image manifest in a remote registry. 
It performs this operation without pulling the image locally.

It is idempotent, fast, and designed for CI/CD automation.

## Features

-   **Efficient:** Updates tags in seconds by transferring only kilobytes of manifest data.
-   **Idempotent:** If the tag already points to the correct image, the tool reports success and does nothing.
-   **Seamless Authentication:** Automatically uses credentials from official login actions for ECR, GCR, Docker Hub, and more.
-   **CI/CD Native:** Provides clear, single-line output, with audit details like creation timestamps, ideal for CI/CD.
-   **Reliable:** Built-in retry mechanism with exponential backoff for transient failures.
-   **Version Pinned:** When using a specific action version (e.g., `@v1.0.0`), the matching binary version is downloaded.

## CLI Usage

The tool can also be used standalone from the command line.

```bash
docker-retag <source-image> <new-tag> [flags]
```

### Flags

| Flag | Description |
|------|-------------|
| `--dry-run` | Validate inputs and check registry connectivity without making changes |
| `--version` | Show version, commit hash, and build time |
| `--help` | Show help message |

### Examples

```bash
# Retag an image
docker-retag myregistry.io/app:build-123 production

# Preview what would happen without making changes
docker-retag --dry-run myregistry.io/app:build-123 production

# Show version info
docker-retag --version
```

## How to Use as a GitHub Action

The primary way to use `docker-retag` is as a step in a GitHub Actions workflow.

### **Core Concept**

The pattern is always the same:
1.  **Log in:** Use an official action from your cloud/registry provider to authenticate. This step configures the runner environment.
2.  **Retag:** Call the `retag` action. It will automatically use the credentials established in the login step.

## Authentication & Usage Examples


### **1. Amazon ECR (Elastic Container Registry)**

#### **Method 1: OIDC (Recommended & Most Secure)**
This method uses short-lived tokens and does not require storing long-lived IAM secrets in GitHub.

**Prerequisites:** You must have an IAM OIDC Provider and an IAM Role configured in your AWS account for GitHub Actions to assume.

```yaml
jobs:
  promote-to-ecr-prod:
    runs-on: ubuntu-latest
    # Permissions are required for the GitHub OIDC provider to get a token
    permissions:
      id-token: write
      contents: read
    steps:
      - name: Configure AWS Credentials via OIDC
        uses: aws-actions/configure-aws-credentials@v4
        with:
          # The ARN of the IAM role to assume
          role-to-assume: arn:aws:iam::123456789012:role/MyGitHubActionsECRRole
          aws-region: us-east-1

      - name: Log in to Amazon ECR
        uses: aws-actions/amazon-ecr-login@v2

      - name: Point :production tag to new build
        uses: abgoyal/docker-retag@v1
        with:
          source_image: 123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app:build-123
          new_tag: production
```

#### **Method 2: Static Access Keys**
A simpler method if OIDC is not set up.

**Prerequisites:** Store your `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` in your GitHub repository's secrets.

```yaml
jobs:
  promote-to-ecr-staging:
    runs-on: ubuntu-latest
    steps:
      - name: Configure AWS Credentials with Access Keys
        uses: aws-actions/configure-aws-credentials@v4
        with:
          aws-access-key-id: ${{ secrets.AWS_ACCESS_KEY_ID }}
          aws-secret-access-key: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          aws-region: us-east-1

      - name: Log in to Amazon ECR
        uses: aws-actions/amazon-ecr-login@v2

      - name: Point :staging tag to new build
        uses: abgoyal/docker-retag@v1
        with:
          source_image: 123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app:build-456
          new_tag: staging
```

### **2. Google Artifact Registry & GCR**

#### **Method: Workload Identity Federation (Recommended)**
This is Google Cloud's equivalent to OIDC for secure, keyless authentication.

**Prerequisites:** You must have a Workload Identity Pool and Service Account configured in your GCP project.

```yaml
jobs:
  promote-to-gcr-prod:
    runs-on: ubuntu-latest
    permissions:
      contents: 'read'
      id-token: 'write'
    steps:
      - name: Authenticate to Google Cloud
        uses: 'google-github-actions/auth@v2'
        with:
          workload_identity_provider: 'projects/1234567890/locations/global/workloadIdentityPools/my-pool/providers/my-provider'
          service_account: 'my-service-account@my-gcp-project.iam.gserviceaccount.com'

      - name: Authorize Docker
        # Configures the gcloud credential helper, which our tool uses
        run: gcloud auth configure-docker us-central1-docker.pkg.dev

      - name: Point :production tag to new build
        uses: abgoyal/docker-retag@v1
        with:
          source_image: us-central1-docker.pkg.dev/my-project/my-repo/my-app:build-789
          new_tag: production
```

### **3. GitHub Container Registry (GHCR)**

This is the simplest case, using the action's built-in `GITHUB_TOKEN`.

```yaml
jobs:
  promote-to-ghcr-latest:
    runs-on: ubuntu-latest
    steps:
      - name: Log in to GitHub Container Registry
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.repository_owner }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Point :latest tag to new build
        uses: abgoyal/docker-retag@v1
        with:
          source_image: ghcr.io/${{ github.repository_owner }}/my-app:build-101
          new_tag: latest
```

### **4. Docker Hub**

**Prerequisites:** Create a [Personal Access Token](https://hub.docker.com/settings/security) on Docker Hub and save it as a GitHub secret (e.g., `DOCKERHUB_TOKEN`).

```yaml
jobs:
  promote-to-dockerhub-latest:
    runs-on: ubuntu-latest
    steps:
      - name: Log in to Docker Hub
        uses: docker/login-action@v3
        with:
          username: ${{ secrets.DOCKERHUB_USERNAME }}
          password: ${{ secrets.DOCKERHUB_TOKEN }}

      - name: Point :latest tag to new build
        uses: abgoyal/docker-retag@v1
        with:
          source_image: my-docker-user/my-app:build-202
          new_tag: latest
```

### **5. Advanced Use Case: Retagging Multiple Images**

Use a `matrix` strategy to retag many images in parallel. The login step runs only once, making it highly efficient.

```yaml
jobs:
  promote-all-services:
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false # Ensures one failure doesn't cancel the others
      matrix:
        image:
          - { source: 'ghcr.io/my-org/service-a:build-101', dest: 'production' }
          - { source: 'ghcr.io/my-org/service-b:build-202', dest: 'production' }
          - { source: 'ghcr.io/my-org/data-processor:build-303', dest: 'production' }

    steps:
      - name: Log in to GitHub Container Registry
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.repository_owner }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Retag ${{ matrix.image.source }} -> ${{ matrix.image.dest }}
        uses: abgoyal/docker-retag@v1
        with:
          source_image: ${{ matrix.image.source }}
          new_tag: ${{ matrix.image.dest }}
```

## For Maintainers: How to Release New Versions

This repository uses `goreleaser` and GitHub Actions to automate releases.

1.  **Develop:** Make changes to the Go source code (`main.go`) or other repository files.
2.  **Commit and Push:** Push your changes to the `main` branch.
3.  **Tag a New Version:** The release workflow is triggered by pushing a new tag that starts with `v` (e.g., `v1.0.1`, `v1.1.0`). To create and push a tag:

    ```bash
    # Create a semantic version tag
    git tag v1.1.0

    # Push the tag to the remote repository
    git push origin v1.1.0
    ```
4.  **Automated Release:** Pushing the tag automatically triggers the "Release Binaries" action. It will build binaries for all target platforms, create a new GitHub Release, and attach the binaries as downloadable assets.

Users of the action can then update their workflows to use the new version (e.g., `uses: abgoyal/retag@v1.1.0`).

