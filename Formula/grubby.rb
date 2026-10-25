class Grubby < Formula
  desc "Terminal cockpit TUI for GitHub repositories, PRs, and workspaces"
  homepage "https://github.com/simonbronner/grubby"
  version "1"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v1/grubby_v1_darwin_arm64.tar.gz"
      sha256 "c1a93dfce9b2c852358de70600c367408d7c9d06151e1255ec83d07c6cab610a"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v1/grubby_v1_darwin_amd64.tar.gz"
      sha256 "e5902a854d776aae1355ff24729dd6016e4b6a6adfade9f612a96b90035c9b8d"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v1/grubby_v1_linux_arm64.tar.gz"
      sha256 "4d7431b6d48357c3bde307144db0eaa354f9f1ee397e444bc92995937eab4d2d"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v1/grubby_v1_linux_amd64.tar.gz"
      sha256 "a1c69248abf632ad123e21fac90cf57f140645933c7d0ca0b2391116194bf50d"
    end
  end

  def install
    bin.install "grubby"
  end

  test do
    assert_match "grubby", shell_output("#{bin}/grubby -v")
  end
end
