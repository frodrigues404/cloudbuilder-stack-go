package cfn

import (
	"strings"

	cft "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
)

func Capabilities(vals []string) []cft.Capability {
	var out []cft.Capability
	for _, v := range vals {
		switch strings.ToUpper(strings.TrimSpace(v)) {
		case "CAPABILITY_IAM":
			out = append(out, cft.CapabilityCapabilityIam)
		case "CAPABILITY_NAMED_IAM":
			out = append(out, cft.CapabilityCapabilityNamedIam)
		case "CAPABILITY_AUTO_EXPAND":
			out = append(out, cft.CapabilityCapabilityAutoExpand)
		}
	}
	return out
}

func Tags(m map[string]string) []cft.Tag {
	if len(m) == 0 {
		return nil
	}
	out := make([]cft.Tag, 0, len(m))
	for k, v := range m {
		kc, vc := k, v
		out = append(out, cft.Tag{Key: &kc, Value: &vc})
	}
	return out
}
