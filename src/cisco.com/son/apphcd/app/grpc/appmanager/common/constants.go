// Author  <dorzheho@cisco.com>

package common

type AppInstanceDataNextAction int8

const (
	AppNameStringLength  = 15
	AppRootGroupIdLength = 20
	AppGroupIdLength     = 15

	TypePeriodic = "periodic"
	TypeDaemon   = "daemon"
	TypeRunOnce  = "run_once"

	TypeDeployment = "deployment"
	TypeCronJob    = "cronJob"
	TypeJob        = "job"

	AttributeAppId     = "app_id"
	AttributeSecretKey = "secret_key"

	AppAnnotationBaseName                     = "apphc.app.basename"
	AppAnnotationCycle                        = "apphc.app.cycle"
	AppAnnotationSchedule                     = "apphc.app.schedule"
	AppInstanceAnnotationRootGroupId          = "apphc.app.instance.root_group_id"
	AppInstanceAnnotationGroupId              = "apphc.app.instance.group_id"
	AppInstanceAnnotationVersion              = "apphc.app.instance.version"
	AppInstanceAnnotationId                   = "apphc.app.instance.id"
	AppInstanceAnnotationTemplateName         = "apphc.app.instance.template_name"
	AppInstanceAnnotationPersistentVolumeSize = "apphc.app.instance.volume.size"
	AppInstanceAnnotationImageRepoName        = "apphc.app.instance.image_repo.name"
	AppInstanceAnnotationImageName            = "apphc.app.instance.image.name"
	AppInstanceAnnotationImageTag             = "apphc.app.instance.image.tag"
	AppInstanceAnnotationState                = "apphc.app.instance.state"
	AppLabelCycle                             = "apphc.app.cycle"
	AppLabelRootGroupId                       = "apphc.app.instance.root_group_id"
	AppLabelGroupId                           = "apphc.app.instance.group_id"

	AppInstanceDataNextActionCreate AppInstanceDataNextAction = iota
	AppInstanceDataNextActionUpgrade
	AppInstanceDataNextActionDelete
	AppInstanceDataNextActionRecreate
	AppInstanceDataNextActionNone

	AppInstanceDefaultCpuRequest          = "0.1"
	AppInstanceDefaultMemoryRequest       = "20Mi"
	AppInstanceDefaultCpuRequestFloat64   = 0.1
	AppInstanceDefaultMemoryRequestUint32 = 20

	EnvVarAppName                = "APP_NAME"
	EnvVarAppStorageDir          = "APP_STORAGE_DIR"
	EnvVarAppInstanceName        = "APP_INSTANCE_NAME"
	EnvVarAppInstanceId          = "APP_INSTANCE_ID"
	EnvVarAppInstanceRootGroupId = "APP_INSTANCE_ROOT_GROUP_ID"
	EnvVarAppInstanceGroupId     = "APP_INSTANCE_GROUP_ID"
	EnvVarAppInstanceVersion     = "APP_INSTANCE_VERSION"
	EnvVarAppInstanceStorageDir  = "APP_INSTANCE_STORAGE_DIR"
	EnvVarAppInstanceSecretsDir  = "APP_INSTANCE_SECRETS_DIR"
	EnvVarAppInstanceConfigDir   = "APP_INSTANCE_CONFIG_DIR"
	EnvVarAppInstanceFlexApiHost = "APP_INSTANCE_FLEX_API_HOST"
	EnvVarAppInstanceFlexApiPort = "APP_INSTANCE_FLEX_API_PORT"
	EnvVarAppInstanceAppId       = "APP_INSTANCE_APP_ID"
	EnvVarAppInstanceSecretKey   = "APP_INSTANCE_SECRET_KEY"

	AppDefaultStorageDir         = "/opt/app/storage/shared"
	AppInstanceDefaultStorageDir = "/opt/app/storage/instance"
	AppInstanceDefaultConfigDir  = "/opt/app/config/"
	AppInstanceDefaultSecretsDir = "/opt/app/secret/"

	DraftAppLabelBasename = "apphc.draft.app.basename" // Required for Draft APPH plugin
	MonAppLabelBasename   = "apphc.mon.app_basename"   // Required by APPH Prometheus design

	TemplatesUrlMonitorAppsDaemon   = "templates.url.monitor.apph.apps.daemon"
	TemplatesUrlMonitorAppsPeriodic = "templates.url.monitor.apph.apps.periodic"
	TemplatesUrlMonitorAppsRunonce  = "templates.url.monitor.apph.apps.runonce"

	CatalogNameTemplates = "templates"
	CatalogTemplatesRepo = "templates"
	CatalogAppsRepo      = "apps"
	CatalogUser          = "catalog"

	WeekDaySunday    = "Sunday"
	WeekDayMonday    = "Monday"
	WeekDayTuesday   = "Tuesday"
	WeekDayWednesday = "Wednesday"
	WeekDayThursday  = "Thursday"
	WeekDayFriday    = "Friday"
	WeekDaySaturday  = "Saturday"
)
