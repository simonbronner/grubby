class Grubby < Formula
  desc "Terminal cockpit TUI for GitHub repositories, PRs, and workspaces"
  homepage "https://github.com/simonbronner/grubby"
  version "15"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v15/grubby_v15_darwin_arm64.tar.gz"
      sha256 "004b6408c470dd34595abcbd8f2946adcb685798ffbc4ddb47b24e90efe320d5"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v15/grubby_v15_darwin_amd64.tar.gz"
      sha256 "4e0393c6c071a094b3b4c5f0ae6998ee7775a6c0a929f713007c5d8117d9f37d"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v15/grubby_v15_linux_arm64.tar.gz"
      sha256 "ebfcf0063dfa1d3541153bf7a6d2ea2ff3d2b7c78c83d58593af41a733f8c54d"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v15/grubby_v15_linux_amd64.tar.gz"
      sha256 "8e89d886719e276088f8a85a0d898bc0c2a24a5178ef616ccbe94e0acedf5b0e"
    end
  end

  def install
    bin.install "grubby"
  end

  test do
    assert_match "grubby", shell_output("#{bin}/grubby -v")
  end
end
