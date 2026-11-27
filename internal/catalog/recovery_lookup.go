package catalog

// ResolveRecoveryAPI maps a public HTTP UUID to a discovery key at the same revision.
func (d *Discovery) ResolveRecoveryAPI(apiID string, revision int) (string, bool) {
	if d == nil || d.snapshot == nil {
		return "", false
	}
	name, ok := d.snapshot.APIByID[apiID]
	if !ok {
		return "", false
	}
	binding := d.snapshot.Tools[name]
	if binding == nil || binding.Revision != revision || d.Version(name) == "" {
		return "", false
	}
	return name, true
}
