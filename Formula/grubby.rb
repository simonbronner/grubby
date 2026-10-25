class Grubby < Formula
  desc "Terminal cockpit TUI for GitHub repositories, PRs, and workspaces"
  homepage "https://github.com/simonbronner/grubby"
  version "8"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v8/grubby_v8_darwin_arm64.tar.gz"
      sha256 "4736192cd057960274df83194b857dc99b85ffbc16f6138619ff87114b38e3b7"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v8/grubby_v8_darwin_amd64.tar.gz"
      sha256 "53774cd398e7d0e23e920eee65c0ea777e119c2b2a5fb9bbc495e38f701454d5"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v8/grubby_v8_linux_arm64.tar.gz"
      sha256 "3294d66db3c4504ede82145904b8680805bcfa32d1ee639ac8dd80815f81af79"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v8/grubby_v8_linux_amd64.tar.gz"
      sha256 "30490cfb6dcf34d492de61d687e18d77c430360d8f169424777ad0362cc32da9"
    end
  end

  def install
    bin.install "grubby"
  end

  test do
    assert_match "grubby", shell_output("#{bin}/grubby -v")
  end
end
