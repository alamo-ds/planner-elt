locals {
  storage_account_name = "stplanner${var.environment}eus002"
  container_name       = "elt-raw-${var.environment}"
}

data "azurerm_client_config" "current" {}

resource "azurerm_resource_group" "rg" {
  name     = "rg-planner-elt-${var.environment}"
  location = var.location
}

resource "azurerm_key_vault" "kv" {
  name                      = "kv-planner-elt-${var.environment}"
  location                  = azurerm_resource_group.rg.location
  resource_group_name       = azurerm_resource_group.rg.name
  tenant_id                 = data.azurerm_client_config.current.tenant_id
  sku_name                  = "standard"
  purge_protection_enabled  = false
  enable_rbac_authorization = true
}

resource "azurerm_storage_account" "adls" {
  name                     = local.storage_account_name
  resource_group_name      = azurerm_resource_group.rg.name
  location                 = azurerm_resource_group.rg.location
  account_tier             = "Standard"
  account_replication_type = "LRS"
  is_hns_enabled           = true
}

resource "azurerm_storage_data_lake_gen2_filesystem" "adls_fs" {
  name               = local.container_name
  storage_account_id = azurerm_storage_account.adls.id
}

resource "azurerm_log_analytics_workspace" "law" {
  name                = "law-planner-elt-${var.environment}"
  location            = azurerm_resource_group.rg.location
  resource_group_name = azurerm_resource_group.rg.name
  sku                 = "PerGB2018"
  retention_in_days   = 30
}

resource "azurerm_container_app_environment" "cae" {
  name                       = "cae-planner-elt-${var.environment}"
  location                   = azurerm_resource_group.rg.location
  resource_group_name        = azurerm_resource_group.rg.name
  log_analytics_workspace_id = azurerm_log_analytics_workspace.law.id
}

resource "azurerm_container_app_job" "job" {
  name                         = "caj-planner-elt-${var.environment}"
  location                     = azurerm_resource_group.rg.location
  resource_group_name          = azurerm_resource_group.rg.name
  container_app_environment_id = azurerm_container_app_environment.cae.id
  replica_timeout_in_seconds   = 600
  replica_retry_limit          = 1
  schedule_trigger_config {
    cron_expression = "0 9 * * *"
  }
  identity {
    type = "SystemAssigned"
  }
  secret {
    name                = "tenant-id"
    key_vault_secret_id = "${azurerm_key_vault.kv.vault_uri}secrets/TENANT-ID"
    identity            = "SystemAssigned"
  }
  secret {
    name                = "client-id"
    key_vault_secret_id = "${azurerm_key_vault.kv.vault_uri}secrets/CLIENT-ID"
    identity            = "SystemAssigned"
  }
  secret {
    name                = "client-secret"
    key_vault_secret_id = "${azurerm_key_vault.kv.vault_uri}secrets/CLIENT-SECRET"
    identity            = "SystemAssigned"
  }

  template {
    container {
      name   = "planner-elt"
      image  = "alamods/planner-elt:${var.image_tag}"
      cpu    = 0.5
      memory = "1Gi"
      env {
        name  = "STORAGE_ACCOUNT_NAME"
        value = azurerm_storage_account.adls.name
      }
      env {
        name  = "BLOB_CONTAINER_NAME"
        value = azurerm_storage_data_lake_gen2_filesystem.adls_fs.name
      }
      env {
        name        = "TENANT_ID"
        secret_name = "tenant-id"
      }
      env {
        name        = "CLIENT_ID"
        secret_name = "client-id"
      }
      env {
        name        = "CLIENT_SECRET"
        secret_name = "client-secret"
      }
    }
  }
}

resource "azurerm_role_assignment" "kv_reader" {
  scope                = azurerm_key_vault.kv.id
  role_definition_name = "Key Vault Secrets User"
  principal_id         = azurerm_container_app_job.job.identity[0].principal_id
}
