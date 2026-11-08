package domain

type Role string

const (
	RoleLead     Role = "lead"
	RoleWorker   Role = "worker"
	RoleReviewer Role = "reviewer"
	RoleExplorer Role = "explorer"
)
