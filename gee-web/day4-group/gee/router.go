package gee

type Router struct {
	trie *Trie
}

func NewRouter() *Router {
	return &Router{trie: NewTrie()}
}

func (r Router) AddRouter(method, path string, handler HandlerFunc) {
	r.trie.Insert(path, handler, &Options{
		"Method": method,
	})
}

func (r Router) Search(method, path string) (HandlerFunc, bool) {
	handler, err := r.trie.Search(method, path)
	if err != nil {
		return handler, false
	}
	return handler, true
}
