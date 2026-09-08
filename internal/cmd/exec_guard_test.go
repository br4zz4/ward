package cmd

import "testing"

func TestExecWouldLeak_env(t *testing.T) {
	if !execWouldLeak([]string{"env"}) {
		t.Errorf("env should be flagged as leaking")
	}
}

func TestExecWouldLeak_printenv(t *testing.T) {
	if !execWouldLeak([]string{"printenv"}) {
		t.Errorf("printenv should be flagged as leaking")
	}
}

func TestExecWouldLeak_export(t *testing.T) {
	if !execWouldLeak([]string{"export"}) {
		t.Errorf("export should be flagged as leaking")
	}
}

func TestExecWouldLeak_declare(t *testing.T) {
	if !execWouldLeak([]string{"declare", "-p"}) {
		t.Errorf("declare -p should be flagged as leaking")
	}
}

func TestExecWouldLeak_set(t *testing.T) {
	if !execWouldLeak([]string{"set"}) {
		t.Errorf("set should be flagged as leaking")
	}
}

func TestExecWouldLeak_echo_var(t *testing.T) {
	if !execWouldLeak([]string{"echo", "$MY_SECRET"}) {
		t.Errorf("echo $VAR should be flagged as leaking")
	}
	if !execWouldLeak([]string{"echo", "${MY_SECRET}"}) {
		t.Errorf("echo ${VAR} should be flagged as leaking")
	}
}

func TestExecWouldLeak_sh_c_echo(t *testing.T) {
	if !execWouldLeak([]string{"sh", "-c", "echo $MY_SECRET"}) {
		t.Errorf("sh -c 'echo $VAR' should be flagged as leaking")
	}
}

func TestExecWouldLeak_sh_c_env(t *testing.T) {
	if !execWouldLeak([]string{"sh", "-c", "env"}) {
		t.Errorf("sh -c 'env' should be flagged as leaking")
	}
	if !execWouldLeak([]string{"bash", "-lc", "printenv"}) {
		t.Errorf("bash -lc 'printenv' should be flagged as leaking")
	}
}

func TestExecWouldLeak_sh_c_compound_echo(t *testing.T) {
	if !execWouldLeak([]string{"sh", "-c", "echo prefix; echo $MY_SECRET"}) {
		t.Errorf("sh -c compound with echo $VAR should be flagged")
	}
	if !execWouldLeak([]string{"sh", "-c", "true && env"}) {
		t.Errorf("sh -c 'true && env' should be flagged")
	}
}

func TestExecWouldLeak_sh_c_test_not_flagged(t *testing.T) {
	// `test "$X" = "y" && echo ok` uses a variable but never prints its value
	if execWouldLeak([]string{"sh", "-c", `test "$REGION" = "us-east-1" && echo ok`}) {
		t.Errorf("sh -c 'test $X && echo ok' should NOT be flagged (value not printed)")
	}
}

func TestExecWouldLeak_cat_proc_environ(t *testing.T) {
	if !execWouldLeak([]string{"cat", "/proc/self/environ"}) {
		t.Errorf("cat /proc/*/environ should be flagged as leaking")
	}
}

func TestExecWouldLeak_normal_command_not_flagged(t *testing.T) {
	if execWouldLeak([]string{"rails", "server"}) {
		t.Errorf("rails server should not be flagged")
	}
	if execWouldLeak([]string{"pytest"}) {
		t.Errorf("pytest should not be flagged")
	}
	if execWouldLeak([]string{"sh", "-c", "ls -la"}) {
		t.Errorf("sh -c 'ls' should not be flagged")
	}
	if execWouldLeak([]string{"curl", "-H", "Authorization: Bearer $API_KEY", "https://api.example.com"}) {
		t.Errorf("curl using a secret should NOT be flagged (it is the intended use)")
	}
}