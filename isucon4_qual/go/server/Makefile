all: template.go resource.rs.go
template.go: template.tmpl
	blackburner -o template.go template.tmpl
resource.rs.go: public/*
	resourceburner -p server resource.json
