package contact

import "testing"

func TestClassifyServiceUsesStableBroadClasses(t *testing.T) {
	tests := []struct {
		name     string
		service  string
		port     uint16
		protocol string
		want     ServiceClass
	}{
		{name: "https", service: "HTTPS", port: 443, want: ServiceClassWeb},
		{name: "redis", service: "redis", port: 6379, want: ServiceClassDatabase},
		{name: "rdp alias", service: "ms-wbt-server", port: 3389, want: ServiceClassRDP},
		{name: "protocol fallback", service: "", port: 9443, protocol: "https", want: ServiceClassWeb},
		{name: "port fallback", service: "", port: 22, want: ServiceClassSSH},
		{name: "unknown", service: "custom-protocol", port: 49152, want: ServiceClassOtherTCP},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ClassifyService(test.service, test.port, test.protocol); got != test.want {
				t.Fatalf("ClassifyService(%q, %d, %q) = %q, want %q", test.service, test.port, test.protocol, got, test.want)
			}
		})
	}
}

func TestClassifyServiceDoesNotTreatUnknownPortAsKnownRateClass(t *testing.T) {
	if got := ClassifyService("", 1, "tcp"); got != ServiceClassOtherTCP {
		t.Fatalf("unknown service was classified as %q", got)
	}
}

func TestIsKnownServiceClassRejectsUnreviewedClass(t *testing.T) {
	if IsKnownServiceClass(ServiceClass("unreviewed-service")) {
		t.Fatal("unreviewed service class was accepted")
	}
	if !IsKnownServiceClass(ServiceClassOtherTCP) {
		t.Fatal("explicit other-tcp fallback was rejected")
	}
}
