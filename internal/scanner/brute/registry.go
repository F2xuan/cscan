package brute

// PluginRegistry 插件注册表
var PluginRegistry = make(map[string]BrutePlugin)

// init 注册所有插件
func init() {
	for name, plugin := range GetAllPlugins() {
		PluginRegistry[name] = plugin
	}
}

// GetPlugin 获取指定服务的插件
func GetPlugin(service string) BrutePlugin {
	switch service {
	case "ssh":
		return &SSHPlugin{}
	case "mysql":
		return &MySQLPlugin{}
	case "redis":
		return &RedisPlugin{}
	case "mongodb":
		return &MongoDBPlugin{}
	case "postgresql":
		return &PostgreSQLPlugin{}
	case "mssql":
		return &MSSQLPlugin{}
	case "ftp":
		return &FTPPlugin{}
	case "smb":
		return &SMBPlugin{}
	case "oracle":
		return &OraclePlugin{}
	case "mqtt":
		return &MQTTPlugin{}
	default:
		return nil
	}
}

// GetAllPlugins 获取所有插件
func GetAllPlugins() map[string]BrutePlugin {
	return map[string]BrutePlugin{
		"ssh":        &SSHPlugin{},
		"mysql":      &MySQLPlugin{},
		"redis":      &RedisPlugin{},
		"mongodb":    &MongoDBPlugin{},
		"postgresql": &PostgreSQLPlugin{},
		"mssql":      &MSSQLPlugin{},
		"ftp":        &FTPPlugin{},
		"smb":        &SMBPlugin{},
		"oracle":     &OraclePlugin{},
		"mqtt":       &MQTTPlugin{},
	}
}
