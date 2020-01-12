# sshw

[![Build Status](https://travis-ci.org/ljun20160606/sshw.svg?branch=master)](https://travis-ci.org/yinheli/sshw)

ssh client wrapper for automatic login.

![usage](./assets/sshw-demo.gif)

## install

use `go get`

```
go get -u github.com/ljun2016060/sshw/cmd/sshw
```

or download binary from [releases](//github.com/ljun20160606/sshw/releases).

## upgrade

show latest version

```shell
sshw latest
```

upgrade sshw

```shell
sshw upgrade
```

## config

put config file in `~/.sshw` or `~/.sshw.yml` or `~/.sshw.yaml` or `./.sshw` or `./.sshw.yml` or `./.sshw.yaml`.
or `sshw -f ./.sshw.yaml` to set filename. With alias `sshw -f ./.sshw.yaml dev` 

<details>
    <summary>config example:</summary>

```yaml
- { name: dev server fully configured, user: appuser, host: 192.168.8.35, port: 22, password: 123456 }
- { name: dev server with key path, user: appuser, host: 192.168.8.35, port: 22, keypath: /root/.ssh/id_rsa }
- { name: dev server with passphrase key, user: appuser, host: 192.168.8.35, port: 22, keypath: /root/.ssh/id_rsa, passphrase: abcdefghijklmn}
- { name: dev server without port, user: appuser, host: 192.168.8.35 }
- { name: dev server without user, host: 192.168.8.35 }
- { name: dev server without password, host: 192.168.8.35 }
- { name: ⚡️ server with emoji name, host: 192.168.8.35 }
- { name: server with alias, alias: dev, host: 192.168.8.35 }
- name: server with jump
  user: appuser
  host: 192.168.8.35
  port: 22
  password: 123456
  jump:
  - user: appuser
    host: 192.168.8.36
    port: 2222


# server group 1
- name: server group 1
  children:
  - { name: server 1, user: root, host: 192.168.1.2 }
  - { name: server 2, user: root, host: 192.168.1.3 }
  - { name: server 3, user: root, host: 192.168.1.4 }

# server group 2
- name: server group 2
  children:
  - { name: server 1, user: root, host: 192.168.2.2 }
  - { name: server 2, user: root, host: 192.168.3.3 }
  - { name: server 3, user: root, host: 192.168.4.4 }
```
</details>

# reserved parameter

`$USER` is used to get `user`

Usage:

```yaml
- { name: dev server fully configured, user: $USER, host: 192.168.8.35 }
```



sshw will use current-user to ssh server

# callback

`callback-shells` is used to run command after ssh open a session. `error-pattern` is regex pattern that
can be used to match message of error, it will return error if match successfully,
but `sshw` receive output async, so if it could not match successfully, you can try wait some time.

`delay` mean before cmd, 'wait' mean after cmd

```
- name: dev server fully configured
  user: appuser
  host: 192.168.8.35
  port: 22
  password: 123456
  callback-shells:
  - {cmd: 2, error-pattern: 'No such file'}
  - {delay: 1500, cmd: 0}
  - {cmd: 'echo 1'}
  - {cmd: 'exit 1', wait: 1000}
```

# ssh-agent

Support [ssh-agent](https://en.wikipedia.org/wiki/Ssh-agent)

Usage:

Add private to keychain

```shell
ssh-add ~/.ssh/.id_rsa
```

When jumper does not support channel, and you has no permission to modify `/etc/ssh/ssh_config`，or you won't save your private id_rsa in jumper. sshw will connect to jumper by ssh-agent.

```yaml
- name: dev server fully configured
  user: appuser
  host: 192.168.8.35
  callback-shells:
  - {cmd: 'ssh 192.168.8.36'}
```

# copy file

Callback support cp file to remote from local, it will convert src file to hex, and echo to target path

```yaml
- name: dev server fully configured
  user: appuser
  host: 192.168.8.35
  callback-shells:
  - { cp: { src: '~/test.txt', tgt: '/tmp/test.txt' } }
```

# execs

Run command before ssh dial or after cancel ssh dial. `execs-pre` is before dial. `execs-stop` is after cancel ssh,
but it's important to note that `execs-stop` only run in host does exist.

If only needs to run command.

```yaml
- name: command-checklist
  execs-pre:
  - cmd: "echo command-checklist"
  - cmd: "something else" 
```

If run command after cancel ssh.

```yaml
- name: tell-some
  host: localhost
  callback-shells:
  - {cmd: 'echo login localhost'}
  - {cmd: 'exit 0'}
  execs-stop:
  - cmd: 'echo exit ssh'
```

# template

Environment variable could be read if the value is string, use `${param1,param2:defaultValue}`, it would be rendered by any one which is not empty value of `param1` `param2`.
Support escape, read as literal `${foo}` if use `\${foo}`.

```yaml
- name: ${name}
  execs-pre:
  - {cmd: 'echo ${content} localhost'}
```

# merge

Merge config A to B.Only compare name and do override, otherwise it is complex.Also can use `-f` to specify persistent config.

```shell
sshw merge ./other.yaml
```