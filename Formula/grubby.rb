class Grubby < Formula
  desc "Terminal cockpit TUI for GitHub repositories, PRs, and workspaces"
  homepage "https://github.com/simonbronner/grubby"
  version "20"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v20/grubby_v20_darwin_arm64.tar.gz"
      sha256 "ca983ab114289a75570b1de46f117007e58e9c23c3978aec322c55ff4db14160"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v20/grubby_v20_darwin_amd64.tar.gz"
      sha256 "d6f6408c5875f228a25e9f08168ca4ce64290c261e659b4f9cd9aa071732b420"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v20/grubby_v20_linux_arm64.tar.gz"
      sha256 "6855574949490ced22c5c0a51e37ff909c5dbf03d7ebe408be73de39a4eaeb53"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v20/grubby_v20_linux_amd64.tar.gz"
      sha256 "ea702ae76cfe8a42995fd12a0faf922aace2775f2ece08161ccb4e7a8813228a"
    end
  end

  def install
    bin.install "grubby"
  end

  test do
    assert_match "grubby", shell_output("#{bin}/grubby -v")
  end
end
