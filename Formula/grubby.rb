class Grubby < Formula
  desc "Terminal cockpit TUI for GitHub repositories, PRs, and workspaces"
  homepage "https://github.com/simonbronner/grubby"
  version "18"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v18/grubby_v18_darwin_arm64.tar.gz"
      sha256 "5e3c76d3b0cb3937647051b42f020647829007ee3d763623fed099a71366beb5"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v18/grubby_v18_darwin_amd64.tar.gz"
      sha256 "0877e6d3334f283879354f53ef55ba6348c116c7e1b7a53ab2aaef4cd3d0f9c3"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v18/grubby_v18_linux_arm64.tar.gz"
      sha256 "5ce9f67cad5a6c687cd37e8b11199845b81f10fdd378a8b8007a175c56afec7a"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v18/grubby_v18_linux_amd64.tar.gz"
      sha256 "8d7b959d00d4a6f4724521033a2b0e7604e1000c2b8511e21586d52d96d3e00d"
    end
  end

  def install
    bin.install "grubby"
  end

  test do
    assert_match "grubby", shell_output("#{bin}/grubby -v")
  end
end
