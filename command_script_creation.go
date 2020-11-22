package webhook_core

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/devops-simba/helpers"
	log "github.com/golang/glog"
)

func createScriptsFolder(command *CLICommand) (string, error) {
	folder, err := os.Getwd()
	if err != nil {
		return "", err
	}

	if command.ScriptFolder != "" {
		folder = filepath.Join(folder, command.ScriptFolder)
	}

	stat, err := os.Stat(folder)
	if os.IsNotExist(err) {
		return folder, os.MkdirAll(folder, os.ModePerm)
	} else if !stat.IsDir() {
		return "", fmt.Errorf("Destination address(%s) is not a folder", command.ScriptFolder)
	}

	return folder, nil
}
func getDockerfilePath(command *CLICommand) (string, error) {
	folder, err := createScriptsFolder(command)
	if err != nil {
		return "", err
	}

	return filepath.Join(folder, "Dockerfile"), nil
}
func getSecretsFolder(command *CLICommand) (string, error) {
	folder, err := createScriptsFolder(command)
	if err != nil {
		return "", err
	}

	folder = filepath.Join(folder, ".secrets")
	if !helpers.PathIsDir(folder) {
		err = os.MkdirAll(folder, os.ModePerm)
		if err != nil {
			return "", err
		}
	}

	return folder, nil
}

func updatePort(command *CLICommand) int {
	if command.Port == 0 {
		// port is automatic
		if command.RunAsUser == 0 {
			if command.Insecure {
				command.Port = 80
			} else {
				command.Port = 443
			}
		} else {
			if command.Insecure {
				command.Port = 8080
			} else {
				command.Port = 8443
			}
		}
	}

	if command.Insecure {
		return 80
	}
	return 443
}
func loadOrCreateCert(command *CLICommand, folder string, ca *helpers.CertAndKey, forceCreate bool) (certAndKey *helpers.CertAndKey, created bool, err error) {
	basename := "ca"
	if ca == nil {
		basename = "srv"
	}

	certFile := filepath.Join(folder, basename+".cert")
	keyFile := filepath.Join(folder, basename+".key")
	if ca != nil {
		command.CertificateFile = certFile
		command.PrivateKeyFile = keyFile
	}
	if !forceCreate {
		if helpers.PathIsFile(certFile) && helpers.PathIsFile(keyFile) {
			certAndKey, err = helpers.LoadCertAndKeyFromCertAndKey(certFile, keyFile)
			return
		}
	}

	commonName := fmt.Sprintf("%s/%s", command.Namespace, command.ApplicationName)
	if ca == nil {
		commonName = "CA for " + commonName
	}

	created = true
	var cert *x509.Certificate
	cert, err = helpers.CreateX509Certificate(commonName, ca == nil, time.Now().AddDate(5, 0, 0))
	if err != nil {
		return
	}

	certAndKey, err = helpers.CreateCertificate(cert, nil, ca)
	if err != nil {
		return
	}

	var block *pem.Block
	block, err = certAndKey.CertificatePEMBlock()
	if err != nil {
		return
	}
	file, err := os.Create(certFile)
	if err != nil {
		return
	}
	err = pem.Encode(file, block)
	file.Close()
	if err != nil {
		return
	}

	block, err = certAndKey.PrivateKeyPEMBlock()
	if err != nil {
		return
	}
	file, err = os.Create(keyFile)
	if err != nil {
		return
	}
	err = pem.Encode(file, block)
	file.Close()
	return
}
func buildTlsKeys(command *CLICommand) (string, error) {
	if command.Insecure {
		if command.CertificateFile != "" || command.PrivateKeyFile != "" || command.CAFile != "" {
			log.Warning("TLS files will be ignored due to --insecure")
		}
		return "", nil
	}

	if command.CertificateFile == "" {
		// no explicit certificate specified, create a random self signed one
		if command.PrivateKeyFile != "" || command.CAFile != "" {
			log.Error("You must either provide all TLS files or none of them")
			return "", errors.New("You must only provide --key/--ca, when you also provide --cert")
		}

		// we must create a certificates as CA for our server
		secretsFolder, err := getSecretsFolder(command)
		if err != nil {
			return "", err
		}

		ca, created, err := loadOrCreateCert(command, secretsFolder, nil, false)
		if err != nil {
			return "", err
		}

		_, _, err = loadOrCreateCert(command, secretsFolder, ca, created)
		if err != nil {
			return "", err
		}

		// now that I have loaded certificates, I should return CA certificate as base64
		block, err := ca.CertificatePEMBlock()
		if err != nil {
			return "", err
		}
		buf := pem.EncodeToMemory(block)
		return base64.StdEncoding.EncodeToString(buf), nil
	} else {
		if command.PrivateKeyFile != "" {
			log.Error("Missing private key file")
			return "", errors.New("Missing private key file")
		}

		_, err := helpers.LoadCertAndKeyFromCertAndKey(command.CertificateFile, command.PrivateKeyFile)
		if err != nil {
			return "", err
		}

		if command.CAFile != "" {
			content, err := ioutil.ReadFile(command.CAFile)
			if err != nil {
				return "", err
			}

			return base64.StdEncoding.EncodeToString(content), nil
		}
		return "", nil
	}
}

// CreateDeployment create deployment scripts in a folder, you may review and modify them and then
// deploy them to the kubernetes
func CreateDeployment(command *CLICommand) error {
	deploymentFolder, err := createScriptsFolder(command)
	if err != nil {
		return err
	}

	// update automatic port
	serverPort := updatePort(command)

	// first of all create Dockerfile
	dockerfilePath := filepath.Join(deploymentFolder, "Dockerfile")
	err = WriteDockerfileToFile(dockerfilePath, DockerfileData{
		BuildProxy:      command.BuildProxy,
		LogLevel:        command.LogLevel,
		Port:            command.Port,
		CertificateFile: fmt.Sprintf("/run/secrets/%s/tls.cert", command.ApplicationName),
		PrivateKeyFile:  fmt.Sprintf("/run/secrets/%s/tls.key", command.ApplicationName),
	})
	if err != nil {
		return err
	}

	// now create deployment
	caBundle, err := buildTlsKeys(command)
	if err != nil {
		return err
	}

	deploymentData := DeploymentData{
		Name:          command.ApplicationName,
		Namespace:     command.Namespace,
		RunAsUser:     command.RunAsUser,
		ImageRegistry: command.ImageRegistry,
		ImageName:     command.ImageName,
		ImageTag:      command.ImageTag,
		ContainerPort: command.Port,
		ServerPort:    serverPort,
		Insecure:      command.Insecure,
		CABundle:      caBundle,
		TlsSecretName: command.SecretName,
		ServiceName:   command.ServiceName,
	}

	for _, hook := range command.Webhooks {
		data := WebhookData{
			Name:           hook.Name(),
			Rules:          hook.Rules(),
			Configurations: hook.Configurations(),
		}
		if hook.Type() == MutatingAdmissionWebhook {
			deploymentData.MutatingWebhooks = append(deploymentData.MutatingWebhooks, data)
		} else {
			deploymentData.ValidatingWebhooks = append(deploymentData.ValidatingWebhooks, data)
		}
	}

	deploymentfilePath := filepath.Join(deploymentFolder, "deployment.yml")
	err = WriteDeploymentToFile(deploymentfilePath, deploymentData)
	if err != nil {
		return err
	}

	// and at last create deploy.sh
	deployScriptData := DeployScriptData{
		Insecure:         command.Insecure,
		Namespace:        command.Namespace,
		TlsSecretName:    command.SecretName,
		CertificateFile:  command.CertificateFile,
		PrivateKeyFile:   command.PrivateKeyFile,
		DeploymentFolder: deploymentFolder,
		ImageRegistry:    command.ImageRegistry,
		ImageName:        command.ImageName,
		ImageTag:         command.ImageTag,
	}

	deployScriptFilePath := "deploy.sh"
	err = WriteDeployScriptToFile(deployScriptFilePath, deployScriptData)
	if err != nil {
		return err
	}

	return nil
}
