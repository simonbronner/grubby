class Grubby < Formula
  desc "Terminal cockpit TUI for GitHub repositories, PRs, and workspaces"
  homepage "https://github.com/simonbronner/grubby"
  version "14"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v14/grubby_v14_darwin_arm64.tar.gz"
      sha256 "da1570531eeb8557fcec6a82997a53c11059a1cb0ff22d59f020d055afdfbff2"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v14/grubby_v14_darwin_amd64.tar.gz"
      sha256 "245b7ac1c30099fae5e8f3f33ab133f94d0178a8b283609538164bc11d25725f"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v14/grubby_v14_linux_arm64.tar.gz"
      sha256 "efedbbf34caeeb84ad48e92357654060aec91eb11597dc5e404733cbc19b3d71"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v14/grubby_v14_linux_amd64.tar.gz"
      sha256 "edd26193a522241658d77ce549aa0873a4d892eef9db10468a802d7cdc318500"
    end
  end

  def install
    bin.install "grubby"
  end

  test do
    assert_match "grubby", shell_output("#{bin}/grubby -v")
  end
end
