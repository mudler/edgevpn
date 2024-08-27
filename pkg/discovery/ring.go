package discovery

type Ring struct {
	Data   []string
	Length int
}

func (r *Ring) Add(s string) {
	if len(r.Data) > 0 {
		// Avoid duplicates of the last item
		if r.Data[len(r.Data)-1] == s {
			return
		}
	}

	if len(r.Data)+1 > r.Length {
		r.Data = r.Data[1:]
	}
	r.Data = append(r.Data, s)
}
