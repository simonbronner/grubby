class Grubby < Formula
  desc "Terminal cockpit TUI for GitHub repositories, PRs, and workspaces"
  homepage "https://github.com/simonbronner/grubby"
  version "18"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v18/grubby_v18_darwin_arm64.tar.gz"
      sha256 "c0c6a312f9567ce667cb9b2104a2d0b5b97647f1bb045e35f4402408bcce289b"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v18/grubby_v18_darwin_amd64.tar.gz"
      sha256 "efc7aaaefd2096bf9021dc614910de7cdf4c757291eae8dfd380dc06faa9b662"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v18/grubby_v18_linux_arm64.tar.gz"
      sha256 "3b22f9261f953a248d88a59f2bebb939823538ddde19709590da45b9b9fdb825"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v18/grubby_v18_linux_amd64.tar.gz"
      sha256 "2ff085a1d62ade239321cee694e19c52fd76b06ac8816b29164060aa83b670f9"
    end
  end

  def install
    bin.install "grubby"
  end

  test do
    assert_match "grubby", shell_output("#{bin}/grubby -v")
  end
end
