class Grubby < Formula
  desc "Terminal cockpit TUI for GitHub repositories, PRs, and workspaces"
  homepage "https://github.com/simonbronner/grubby"
  version "2"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v2/grubby_v2_darwin_arm64.tar.gz"
      sha256 "b97bfe62e8744ecf26b836302138e49bc351fb8f0a2136a9838930d50d4cf2ae"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v2/grubby_v2_darwin_amd64.tar.gz"
      sha256 "ba868b6cd510bcdf6e82d3a7b5c33204db9123276e26c55f0eb337b803cc728d"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v2/grubby_v2_linux_arm64.tar.gz"
      sha256 "303379ca5a4bede162dae652e08279b62fe407015c60cbe16c9e3f1f96f0409d"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v2/grubby_v2_linux_amd64.tar.gz"
      sha256 "90010577c1bf646f2fd76d3961c58c18a110e027b78978a35911f7689dcde3e7"
    end
  end

  def install
    bin.install "grubby"
  end

  test do
    assert_match "grubby", shell_output("#{bin}/grubby -v")
  end
end
