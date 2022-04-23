// Copyright 2020 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package cmd create a root cobra command and add subcommands to it.
package cmd

import (
	"flag"
	"io"
	"os"

	cliflag "github.com/marmotedu/component-base/pkg/cli/flag"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/marmotedu/iam/internal/iamctl/cmd/color"
	"github.com/marmotedu/iam/internal/iamctl/cmd/completion"
	"github.com/marmotedu/iam/internal/iamctl/cmd/info"
	"github.com/marmotedu/iam/internal/iamctl/cmd/jwt"
	"github.com/marmotedu/iam/internal/iamctl/cmd/new"
	"github.com/marmotedu/iam/internal/iamctl/cmd/options"
	"github.com/marmotedu/iam/internal/iamctl/cmd/policy"
	"github.com/marmotedu/iam/internal/iamctl/cmd/secret"
	"github.com/marmotedu/iam/internal/iamctl/cmd/set"
	"github.com/marmotedu/iam/internal/iamctl/cmd/user"
	cmdutil "github.com/marmotedu/iam/internal/iamctl/cmd/util"
	"github.com/marmotedu/iam/internal/iamctl/cmd/validate"
	"github.com/marmotedu/iam/internal/iamctl/cmd/version"
	"github.com/marmotedu/iam/internal/iamctl/util/templates"
	genericapiserver "github.com/marmotedu/iam/internal/pkg/server"
	"github.com/marmotedu/iam/pkg/cli/genericclioptions"
)

// NewDefaultIAMCtlCommand creates the `iamctl` command with default arguments.
func NewDefaultIAMCtlCommand() *cobra.Command {
	return NewIAMCtlCommand(os.Stdin, os.Stdout, os.Stderr)
}

// 定义root命令
// 添加命令行选项
// NewIAMCtlCommand returns new initialized instance of 'iamctl' root command.
func NewIAMCtlCommand(in io.Reader, out, err io.Writer) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:   "iamctl",
		Short: "iamctl controls the iam platform",
		Long: templates.LongDesc(`
		iamctl controls the iam platform, is the client side tool for iam platform.

		Find more information at:
			https://github.com/marmotedu/iam/blob/master/docs/guide/en-US/cmd/iamctl/iamctl.md`),
		Run: runHelp,
		// Hook before and after Run initialize and write profiles to disk,
		// respectively.
		PersistentPreRunE: func(*cobra.Command, []string) error {
			return initProfiling()
		},
		PersistentPostRunE: func(*cobra.Command, []string) error {
			return flushProfiling()
		},
	}
	// 创建持久化参数, 返回用于定义flag的flagsets实例
	flags := cmds.PersistentFlags()
	// 设置标准化参数名称的函数, 用于转义flag name
	flags.SetNormalizeFunc(cliflag.WarnWordSepNormalizeFunc) // Warn for "_" flags

	// Normalize all flags that are coming from other packages or pre-configurations
	// a.k.a. change all "_" to "-". e.g. glog package
	flags.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)

	// 添加pprof相关操作的flag
	addProfilingFlags(flags)

	// NewConfigFlags(true) 返回带有默认值的参数，并通过 iamConfigFlags.AddFlags(flags) 添加到cobra的命令行flag中。
	iamConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag().WithDeprecatedSecretFlag()
	iamConfigFlags.AddFlags(flags)
	// NewMatchVersionFlags 指定是否需要服务端版本和客户端版本一致。如果不一致，在调用服务接口时会报错。
	matchVersionIAMConfigFlags := cmdutil.NewMatchVersionFlags(iamConfigFlags)
	matchVersionIAMConfigFlags.AddFlags(cmds.PersistentFlags())

	// iamctl需要连接iam-apiserver，来完成用户、策略和密钥的增删改查，并且需要进行认证。要完成这些功能，需要有比较多的配置项。
	// 这些配置项如果每次都在命令行选项指定，会很麻烦，也容易出错。
	// 最好的方式是保存到配置文件中，并加载配置文件。加载配置文件的代码位于NewIAMCtlCommand函数中.
	// 将标志绑定到Viper，这样就可以使用viper.Get()获取标志的值
	_ = viper.BindPFlags(cmds.PersistentFlags())
	// viper中存储的配置，是在cobra命令启动时通过LoadConfig函数加载的，代码如下（位于 NewIAMCtlCommand 函数中）：
	// 你可以通过 --iamconfig 选项，指定配置文件的路径。viper优先使用命令行指定的配置文件
	cobra.OnInitialize(func() {
		genericapiserver.LoadConfig(viper.GetString(genericclioptions.FlagIAMConfig), "config")
	})
	// 将GO自带的flag库中的指令添加到cobra中，即混用 flag 及 pflag
	cmds.PersistentFlags().AddGoFlagSet(flag.CommandLine)

	f := cmdutil.NewFactory(matchVersionIAMConfigFlags)

	// From this point and forward we get warnings on flags that contain "_" separators
	cmds.SetGlobalNormalizationFunc(cliflag.WarnWordSepNormalizeFunc)

	ioStreams := genericclioptions.IOStreams{In: in, Out: out, ErrOut: err}

	// 将命令分组
	groups := templates.CommandGroups{
		{
			Message: "Basic Commands:",
			Commands: []*cobra.Command{
				info.NewCmdInfo(f, ioStreams),
				color.NewCmdColor(f, ioStreams),
				new.NewCmdNew(f, ioStreams),
				jwt.NewCmdJWT(f, ioStreams),
			},
		},
		{
			Message: "Identity and Access Management Commands:",
			Commands: []*cobra.Command{
				user.NewCmdUser(f, ioStreams),
				secret.NewCmdSecret(f, ioStreams),
				policy.NewCmdPolicy(f, ioStreams),
			},
		},
		{
			Message: "Troubleshooting and Debugging Commands:",
			Commands: []*cobra.Command{
				validate.NewCmdValidate(f, ioStreams),
			},
		},
		{
			Message: "Settings Commands:",
			Commands: []*cobra.Command{
				set.NewCmdSet(f, ioStreams),
				completion.NewCmdCompletion(ioStreams.Out, ""),
			},
		},
	}
	// 将分组后的命令添加到
	groups.Add(cmds)

	filters := []string{"options"}
	templates.ActsAsRootCommand(cmds, filters, groups...)

	cmds.AddCommand(version.NewCmdVersion(f, ioStreams))
	cmds.AddCommand(options.NewCmdOptions(ioStreams.Out))

	return cmds
}

func runHelp(cmd *cobra.Command, args []string) {
	_ = cmd.Help()
}
