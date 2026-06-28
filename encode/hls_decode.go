package encode

// OnDemandHLSDecodeProfile tunes decode for independent short HLS segments.
// Software decode + hardware encode outperforms full HW pipelines on many Intel
// iGPUs where QSV/VAAPI device init dominates 4s segment latency.
func OnDemandHLSDecodeProfile(profile VideoDecodeProfile) VideoDecodeProfile {
	if profile.ForceSoftware {
		return profile
	}
	profile.ForceSoftware = true
	profile.Accel = ""
	profile.Decoder = ""
	return profile
}
