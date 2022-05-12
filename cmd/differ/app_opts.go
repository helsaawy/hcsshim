package main

type reExecOpts func(*reExecConfig) error

// reExecOpts are options to change how a subcommand is re-execed
type reExecConfig struct {
	keepPrivleges []string
}

var defaultPrivileges = withPrivileges([]string{"SeChangeNotifyPrivilege"})

func withPrivileges(keep []string) reExecOpts {
	return func(o *reExecConfig) error {
		o.keepPrivleges = append(o.keepPrivleges, keep...)
		return nil
	}
}
