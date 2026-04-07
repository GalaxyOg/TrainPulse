package context

import "testing"

func TestInferJobName(t *testing.T) {
	cases := []struct {
		cmd  []string
		want string
	}{
		{[]string{"python", "train.py", "--x", "1"}, "train"},
		{[]string{"uv", "run", "python", "foo/bar.py"}, "bar"},
		{[]string{"conda", "run", "-n", "x", "python", "algo.py"}, "algo"},
		{[]string{"bash", "run.sh"}, "bash"},
	}
	for _, tc := range cases {
		got := InferJobName(tc.cmd)
		if got != tc.want {
			t.Fatalf("InferJobName(%v)=%q want %q", tc.cmd, got, tc.want)
		}
	}
}
