Vagrant.configure("2") do |config|
  config.vm.box = "ubuntu/xenial64"

  config.vm.provider 'virtualbox' do |v|
    v.memory = 2048
    v.cpus = 4
  end

  config.vm.synced_folder ".", "/capsule8"
end
