package catalog

// PinnedOpens reports how many generation databases this process has opened and
// pinned. A request served from the generation memo does not add to it.
func PinnedOpens() int64 { return pinnedOpens.Load() }
