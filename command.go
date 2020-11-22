package webhook_core

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/devops-simba/helpers"
)

var (
	InvalidWebhookType = errors.New("Invalid webhook type")
)

type CommandHandler func(command *CLICommand) error

type CLICommand struct {
	// Port port that we should listen on it for incoming requests,
	// default is 443 if this is a secure server and 80 otherwise
	Port int
	// Host host that server should listen on it
	Host string
	// Should we deploy this server as insecure? default is false
	Insecure bool
	// CertificateFile file that contains certificate of the server
	CertificateFile string
	// PrivateKeyFile file that contains key of the
	PrivateKeyFile string

	// CAFile file that contains CA information for certificate
	CAFile string
	// LogLevel level that should used for logging in the docker image
	LogLevel int
	// BuildProxy proxy that we should use to build go application
	BuildProxy string
	// ImageName name of the deployed image, default is name of the folder
	ImageName string
	// ImageTag tag of the deployed image
	ImageTag string
	// ImageRegistry registry that image should pushed to it, default is default registry of the system
	ImageRegistry string
	// RunAsUser identifier of the
	RunAsUser int
	// Kubernetes namespace that pod should deployed to it
	Namespace string
	// SecretName name of the secret that hold certificate of the pod
	SecretName string
	// ServiceName name of the service that should handle this webhook
	ServiceName string
	// Command command that must executed by the program
	Command string
	// Folder that deployment scripts should added to it
	ScriptFolder string

	// ApplicationName name of this application
	ApplicationName string
	// SupportedCommands commands that are supported by the application
	SupportedCommands map[string]CommandHandler
	// DefaultCommand name of default command
	DefaultCommand string
	// Webhooks list of webhooks that defined in this application
	Webhooks []AdmissionWebhook
}

func ReadCommand(
	defaultCommand string,
	supportedCommands map[string]CommandHandler,
	dontAddDefaultCommands bool,
	webhooks ...AdmissionWebhook) *CLICommand {
	if len(webhooks) == 0 {
		panic("Missing any webhook")
	}

	command := &CLICommand{
		Webhooks:          webhooks,
		SupportedCommands: make(map[string]CommandHandler),
		ApplicationName:   strings.Replace(helpers.ApplicationName, "_", "-", -1),
	}

	for key, value := range supportedCommands {
		command.SupportedCommands[key] = value
	}
	if !dontAddDefaultCommands {
		if _, ok := command.SupportedCommands["run"]; !ok {
			command.SupportedCommands["run"] = RunWebhooks
		}
		if _, ok := command.SupportedCommands["deploy"]; !ok {
			command.SupportedCommands["deploy"] = CreateDeployment
		}

		if defaultCommand == "" {
			defaultCommand = "run"
		}
	}
	if _, ok := command.SupportedCommands[defaultCommand]; !ok && defaultCommand != "" {
		panic("Invalid default command")
	}

	command.BindToFlags(flag.CommandLine)
	flag.Parse()

	if command.ServiceName == "" {
		command.ServiceName = command.ApplicationName
	}
	if command.SecretName == "" {
		command.SecretName = command.ApplicationName
	}
	if command.ImageName == "" {
		command.ImageName = command.ApplicationName
	}

	return command
}

func (this *CLICommand) GetSupportedCommandNames() []string {
	result := make([]string, 0, len(this.SupportedCommands))
	for key := range this.SupportedCommands {
		result = append(result, key)
	}
	return result
}
func (this *CLICommand) BindToFlags(flagset *flag.FlagSet) {
	supportedCommands := "[" + strings.Join(this.GetSupportedCommandNames(), ", ") + "]"

	flagset.StringVar(&this.ApplicationName, "app", this.ApplicationName, "Name of the application")
	flagset.IntVar(&this.Port, "port", 0, "Port that server should listen on it")
	flagset.StringVar(&this.Host, "host", "0.0.0.0", "Host that server should listen on it")
	flagset.IntVar(&this.LogLevel, "level", 0, "Level of log information")
	flagset.BoolVar(&this.Insecure, "insecure", false, "Should we run this server as an insecure one?")
	flagset.StringVar(&this.CertificateFile, "cert", "",
		"Path to file that contains certificate of the server(Used in TLS)")
	flagset.StringVar(&this.PrivateKeyFile, "key", "",
		"Path to file that contains private key of the server(Used in TLS)")
	flagset.StringVar(&this.CAFile, "ca", "", "Path to CA that signed certificate of this server")
	flagset.StringVar(&this.ImageName, "image", "", "Name of the docker image")
	flagset.StringVar(&this.ImageTag, "tag", "latest", "Tag of the docker image")
	flagset.StringVar(&this.BuildProxy, "proxy", "",
		"Proxy that we should use to download required go packages when building application")
	flagset.StringVar(&this.ImageRegistry, "registry", "",
		"Registry that image must pushed to it, if it is default just pass an empty string")
	flagset.IntVar(&this.RunAsUser, "runas", 1234,
		"Identifier of the user that application must run under it")
	flagset.StringVar(&this.Namespace, "namespace", "devops-webhooks",
		"Namespace that pod must deployed into it")
	flagset.StringVar(&this.SecretName, "secret-name", "",
		"Name of the secret that contains TLS information of the server")
	flagset.StringVar(&this.ServiceName, "service-name", "",
		"Name of the service that wrap created pod(s)")
	flagset.StringVar(&this.ScriptFolder, "folder", "deployment-scripts",
		"Folder that deployment scripts will be created in it")
	flagset.StringVar(&this.Command, "command", this.DefaultCommand,
		"Command that must executed in current execution. Available commands are: "+supportedCommands)
}

func (this *CLICommand) Execute() error {
	if !flag.Parsed() {
		return errors.New("You should only call this after calling flag.Parse")
	}

	command, ok := this.SupportedCommands[this.Command]
	if !ok {
		return fmt.Errorf("Command '%s' is not supported", this.Command)
	}

	return command(this)
}
