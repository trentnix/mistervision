package rendering

// OptionsMenu is a copied snapshot of media actions. The browser owns selection,
// network requests, and application of changes. Rendering performs no actions.
type OptionsMenu struct {
	Title, Message string
	Rows           []string
	Selected       int
	Busy           bool
}
