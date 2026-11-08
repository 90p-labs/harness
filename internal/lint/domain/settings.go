package domain

type Settings struct {
	DDD           DDD
	Shadcn        Shadcn
	Observability Observability
}

type DDD struct {
	Root        string
	DomainAllow []string
	SrcFiles    []string
}

type Shadcn struct {
	UIAllow []string
}

type Observability struct {
	Chart      string
	Rules      string
	Dashboards string
}
