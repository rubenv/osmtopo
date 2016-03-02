# -*- mode: ruby -*-
# vi: set ft=ruby :

Vagrant.configure(2) do |config|
  config.vm.box = "puppetlabs/centos-7.2-64-nocm"
  config.vm.synced_folder "#{ENV["GOPATH"]}/src", "/go/src", id: "go", nfs: true, mount_options: ["nolock,vers=3,udp,noatime,actimeo=1"]

  config.vm.provision "shell", inline: <<-SHELL
    yum -y install docker
    systemctl enable docker
    systemctl start docker

    echo "export GOPATH=/go" > /etc/profile.d/gopath.sh
  SHELL

  config.vm.provider :vmware_fusion  do |v|
    v.vmx["memsize"] = "8192"
    v.vmx["numvcpus"] = "8"
  end
end