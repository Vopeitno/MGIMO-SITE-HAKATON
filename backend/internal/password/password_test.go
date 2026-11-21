package password

import "testing"

func TestArgon2id(t *testing.T) {
	a, e := Hash("synthetic-secret")
	if e != nil {
		t.Fatal(e)
	}
	b, e := Hash("synthetic-secret")
	if e != nil {
		t.Fatal(e)
	}
	if a == b {
		t.Fatal("соль должна быть случайной")
	}
	if !Verify("synthetic-secret", a) || Verify("wrong", a) {
		t.Fatal("ошибка проверки пароля")
	}
	for _, bad := range []string{"", "plaintext", "$argon2id$v=19$m=4294967295,t=3,p=2$AAAA$AAAA", "$argon2id$v=19$m=0,t=0,p=0$AAAA$AAAA"} {
		if Verify("secret", bad) {
			t.Fatal("невалидный хеш принят")
		}
	}
}
