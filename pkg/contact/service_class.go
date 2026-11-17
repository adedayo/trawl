package contact

import "strings"

// ServiceClass is the taxonomy key used by the contact-rate model. It is
// intentionally broader than a protocol name so model-pack rates remain
// stable when discovery labels the same service in different ways.
type ServiceClass string

const (
	ServiceClassWeb         ServiceClass = "web"
	ServiceClassSSH         ServiceClass = "ssh"
	ServiceClassRDP         ServiceClass = "rdp"
	ServiceClassRemoteAdmin ServiceClass = "remote-admin"
	ServiceClassMail        ServiceClass = "mail"
	ServiceClassDNS         ServiceClass = "dns"
	ServiceClassDatabase    ServiceClass = "database"
	ServiceClassOtherTCP    ServiceClass = "other-tcp"
)

func IsKnownServiceClass(serviceClass ServiceClass) bool {
	switch serviceClass {
	case ServiceClassWeb, ServiceClassSSH, ServiceClassRDP, ServiceClassRemoteAdmin,
		ServiceClassMail, ServiceClassDNS, ServiceClassDatabase, ServiceClassOtherTCP:
		return true
	default:
		return false
	}
}

// ClassifyService maps the service labels emitted by discovery to the
// contact model taxonomy. Unknown labels are deliberately conservative: they
// remain classifiable for inventory coverage but do not inherit a more
// specific service's ambient rate.
func ClassifyService(service string, port uint16, protocol string) ServiceClass {
	value := strings.ToLower(strings.TrimSpace(service))
	protocolValue := strings.ToLower(strings.TrimSpace(protocol))
	switch value {
	case "http", "https", "http-alt", "http-proxy", "web", "web-admin":
		return ServiceClassWeb
	case "ssh":
		return ServiceClassSSH
	case "rdp", "ms-wbt-server":
		return ServiceClassRDP
	case "vpn", "ipsec", "openvpn", "wireguard", "admin", "management":
		return ServiceClassRemoteAdmin
	case "smtp", "smtps", "submission", "imap", "imaps", "pop3", "pop3s", "mail":
		return ServiceClassMail
	case "dns", "domain":
		return ServiceClassDNS
	case "postgres", "postgresql", "mysql", "mariadb", "redis", "mongodb", "mongo", "database":
		return ServiceClassDatabase
	}

	if protocolValue == "http" || protocolValue == "https" {
		return ServiceClassWeb
	}
	// Port fallbacks are limited to unambiguous conventional services. They
	// improve coverage without turning arbitrary port numbers into evidence.
	switch port {
	case 22:
		return ServiceClassSSH
	case 25, 465, 587, 993, 995:
		return ServiceClassMail
	case 53:
		return ServiceClassDNS
	case 3389:
		return ServiceClassRDP
	case 3306, 5432, 6379, 27017:
		return ServiceClassDatabase
	case 80, 443, 8080, 8443:
		return ServiceClassWeb
	default:
		return ServiceClassOtherTCP
	}
}
