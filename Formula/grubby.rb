class Grubby < Formula
  desc "Terminal cockpit TUI for GitHub repositories, PRs, and workspaces"
  homepage "https://github.com/simonbronner/grubby"
  version "13"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v13/grubby_v13_darwin_arm64.tar.gz"
      sha256 "11bfa53f65f742210a9b4977aebc5d5fddf3b3506e8f0006ae01f93c4b5ea4af"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v13/grubby_v13_darwin_amd64.tar.gz"
      sha256 "bb897d38188b35345504376ec2af802c3f49795351c5e6409fde3c0f29e706c2"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v13/grubby_v13_linux_arm64.tar.gz"
      sha256 "6a6672bf0934eec30c601968b3ba4b2cf399ad86bc847e0a98155cce05d9ce33"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v13/grubby_v13_linux_amd64.tar.gz"
      sha256 "7c023d12b532e5288216369be614d1b5deef9c3254ba41951b4ac94ffbea2624"
    end
  end

  def install
    bin.install "grubby"
  end

  test do
    assert_match "grubby", shell_output("#{bin}/grubby -v")
  end
end
