# Kill-Snap Project Context

## Deployment

**NEVER use `sam deploy` directly. ALWAYS use the GitHub Actions pipeline for deployments.**

Push changes to the repository and let the CI/CD pipeline handle deployment.

## UI Terminology

- **Thumbnail Images**: The images displayed in the grid on the Image Review page (ImageGallery component)
- **Review Image**: The larger image shown in the popup modal (ImageModal component) when clicking a thumbnail

## Component Mapping

| Term | Component | Description |
|------|-----------|-------------|
| Image Review Page | `ImageGallery.tsx` | Main page with grid of thumbnail images |
| Thumbnail Images | Grid items in `ImageGallery.tsx` | Small preview images in the gallery grid |
| Review Image | `ImageModal.tsx` | Large image popup for detailed review and actions |

## AWS Deployment Policy

**CRITICAL: All AWS infrastructure and code changes MUST be deployed via GitHub Actions pipelines.**

### Prohibited Actions
- **NEVER** use AWS CLI directly to deploy, update, or modify infrastructure
- **NEVER** use AWS SAM CLI (`sam deploy`, `sam build`, etc.) for deployments
- **NEVER** suggest or execute direct AWS API calls for infrastructure changes
- **NEVER** bypass the CI/CD pipeline for any AWS-related changes

### Required Workflow
1. All changes must be committed and pushed to the repository
2. GitHub Actions pipeline will handle all deployments
3. **ALWAYS review pipeline output** after pushing changes
4. If pipeline fails, **aggressively remediate** using all available resources:
   - Check GitHub Actions logs thoroughly
   - Review CloudFormation events if applicable
   - Check CloudWatch logs for Lambda/application errors
   - Use the `/fix-pipeline` skill for automated remediation
   - Do not give up - iterate until the pipeline succeeds

### Pipeline Failure Remediation
When a GitHub Actions pipeline fails:
1. Immediately fetch and analyze the failure logs
2. Identify the root cause from error messages
3. Make necessary code/configuration fixes
4. Commit and push the fix
5. Monitor the new pipeline run
6. Repeat until successful deployment
