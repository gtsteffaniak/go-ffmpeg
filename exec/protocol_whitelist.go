package exec

// DefaultProtocolWhitelist is applied to ffmpeg/ffprobe before caller-supplied inputs.
const DefaultProtocolWhitelist = "file,pipe"

// DisableProtocolWhitelist is the Config value that skips -protocol_whitelist.
const DisableProtocolWhitelist = "-"

// AppendProtocolWhitelistArgs appends -protocol_whitelist when enabled on r.
// A nil runner uses DefaultProtocolWhitelist.
func (r *Runner) AppendProtocolWhitelistArgs(args []string) []string {
	if r != nil && r.ProtocolWhitelist == DisableProtocolWhitelist {
		return args
	}
	wl := DefaultProtocolWhitelist
	if r != nil && r.ProtocolWhitelist != "" && r.ProtocolWhitelist != DisableProtocolWhitelist {
		wl = r.ProtocolWhitelist
	}
	return append(args, "-protocol_whitelist", wl)
}
