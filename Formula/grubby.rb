class Grubby < Formula
  desc "Terminal cockpit TUI for GitHub repositories, PRs, and workspaces"
  homepage "https://github.com/simonbronner/grubby"
  version "21"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v21/grubby_v21_darwin_arm64.tar.gz"
      sha256 "7f220c2478e340e484b6e56af638ce7acfe8512c9fdab26e35b069bd1ff4fc07"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v21/grubby_v21_darwin_amd64.tar.gz"
      sha256 "59cd0d9ef37658d61bfc95cb43fb00d8ff49566420dba876bc8c60bd7332773d"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v21/grubby_v21_linux_arm64.tar.gz"
      sha256 "af638befadf2b794373beb5d69d78275eeaaabedd3f9f47a2f776d663a4b7690"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v21/grubby_v21_linux_amd64.tar.gz"
      sha256 "7b210a01c0188a6a3025f2a590d588142ad98eca8bc48bf54fedf20d0728f506"
    end
  end

  def install
    bin.install "grubby"
  end

  test do
    assert_match "grubby", shell_output("#{bin}/grubby -v")
  end
end
