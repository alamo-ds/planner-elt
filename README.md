# Planner ELT

Planner ELT is a Go-based Extract, Load, Transform (ELT) tool designed to extract tasks from Microsoft Planner using the Microsoft Graph API and load them into Azure Blob Storage / Azure Data Lake Storage (ADLS).

## Features

- Authenticates with Azure Active Directory (Azure AD) using Client Credentials.
- Fetches tasks from Microsoft Planner via the MS Graph API.
- Pushes the extracted tasks as blobs to Azure Blob Storage.
- Packaged as a Docker container for easy deployment and execution.

## Prerequisites

To run this application, you will need an Azure AD App Registration with the appropriate Microsoft Graph API permissions to read Planner data, as well as access to an Azure Storage Account.

You must provide the following environment variables:

- `tenantid`: Your Azure AD Tenant ID.
- `clientid`: Your Azure AD Application (Client) ID.
- `clientsecret`: Your Azure AD Application Client Secret.

*Note: Additional environment variables may be required for Azure Blob Storage authentication depending on your environment (e.g., `AZURE_STORAGE_ACCOUNT`, `AZURE_STORAGE_KEY`, or using Azure Identity).*

## Running the Application

### Using Docker

The application is published as a Docker image to Docker Hub: `alamods/planner-elt`.

```bash
docker run --rm \
  -e tenantid="YOUR_TENANT_ID" \
  -e clientid="YOUR_CLIENT_ID" \
  -e clientsecret="YOUR_CLIENT_SECRET" \
  alamods/planner-elt:latest
```

### Using Go

Make sure you have Go 1.25+ installed.

```bash
# Clone the repository
git clone https://github.com/alamo-ds/planner-elt.git
cd planner-elt

# Export required environment variables
export tenantid="YOUR_TENANT_ID"
export clientid="YOUR_CLIENT_ID"
export clientsecret="YOUR_CLIENT_SECRET"

# Run the application
go run .
```

## Development

### Building the Docker Image

You can build the Docker image locally using the provided script:

```bash
./scripts/build-img.sh [tag]
```

### CI/CD

This project uses GitHub Actions for Continuous Integration and Deployment:
- **CI**: Runs tests and linters on pull requests and pushes to the main branch.
- **Release**: Automatically builds and pushes the Docker image to Docker Hub and creates a GitHub Release using GoReleaser when a new tag is pushed.

## License

Please refer to the [LICENSE](LICENSE) file in the repository for more information.