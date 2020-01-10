Pod::Spec.new do |spec|
  spec.name         = 'Gccm'
  spec.version      = '{{.Version}}'
  spec.license      = { :type => 'GNU Lesser General Public License, Version 3.0' }
  spec.homepage     = 'https://github.com/ccmchain/go-ccmchain'
  spec.authors      = { {{range .Contributors}}
		'{{.Name}}' => '{{.Email}}',{{end}}
	}
  spec.summary      = 'iOS Ccmchain Client'
  spec.source       = { :git => 'https://github.com/ccmchain/go-ccmchain.git', :commit => '{{.Commit}}' }

	spec.platform = :ios
  spec.ios.deployment_target  = '9.0'
	spec.ios.vendored_frameworks = 'Frameworks/Gccm.framework'

	spec.prepare_command = <<-CMD
    curl https://gccmstore.blob.core.windows.net/builds/{{.Archive}}.tar.gz | tar -xvz
    mkdir Frameworks
    mv {{.Archive}}/Gccm.framework Frameworks
    rm -rf {{.Archive}}
  CMD
end
