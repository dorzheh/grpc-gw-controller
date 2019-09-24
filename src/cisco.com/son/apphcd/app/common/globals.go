// Author  <dorzheho@cisco.com>

package common

import (
	"os"
	"path/filepath"
)

// Environment variables
const (
	EnvApphSvcsUrlExternalIp             = "svcs_url_external_ip" // Apphoster external IP/FQDN. Requires in case Apphoster located behind NAT
	EnvApphExternalIp                    = "external_ip"          // Requires if controller runs outside of the Apphoster cluster
	EnvApphMasterNodeIp                  = "master_node_ip"
	EnvApphMasterNodeUser                = "master_node_user"
	EnvApphcLogFormat                    = "log_format"
	EnvApphcNetworkPort                  = "network_port"
	EnvApphcCachePath                    = "cache_path"
	EnvApphcInternalAuthorizationEnabled = "internal_authorization_enabled"
	EnvApphcBearerToken                  = "bearer_token"
	EnvApphcPrivateDockerRegistry        = "private_docker_registry"
	EnvApphcAppsUpgradePolicyRecreate    = "apps_upgrade_policy_recreate"
	EnvApphcAppsRollbackEnabled          = "apps_upgrade_rollback_enabled"
	EnvApphcAppFlexApiHost               = "flex_api_host"
	EnvApphcAppFlexApiPort               = "flex_api_port"
	EnvApphcGitServerEndpoint            = "git_server_endpoint"
	EnvApphcAdaptersRancherEnabled       = "adapters_rancher_enabled"
	EnvApphcPurgeAppMetadata             = "purge_application_metadata"
)

type LogFormat string

const (
	LogFormatText LogFormat = "text"
	LogFormatJson LogFormat = "json"
)

const (
	AppdataPath       = "/appdata"
	ApphNodeRoleLabel = "apph.node.role"
)

var (
	ApphcHomePath       = filepath.Join(os.Getenv("HOME"), ".apphc")
	ApphcKubeconfigPath = filepath.Join(ApphcHomePath, "kubeconfig")
)
