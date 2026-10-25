class Grubby < Formula
  desc "Terminal cockpit TUI for GitHub repositories, PRs, and workspaces"
  homepage "https://github.com/simonbronner/grubby"
  version "4"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v4/grubby_v4_darwin_arm64.tar.gz"
      sha256 "d4604f2feea2e4d6853f053333c955e21b75d7c56fd2575d1c66b11cedd25edb"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v4/grubby_v4_darwin_amd64.tar.gz"
      sha256 "4dc343096b2a52967a96792dd89b9b89604c223b01fb45ed70e7b003756f2172"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v4/grubby_v4_linux_arm64.tar.gz"
      sha256 "16ce39e5194d75657b7a5844eb99b17879e6ba372f26a1d1237f10ba64a34f06"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v4/grubby_v4_linux_amd64.tar.gz"
      sha256 "2efc7255a0dcb0d4ce99112583f15f227417679f666d4b4320546a98f05a69c0"
    end
  end

  def install
    bin.install "grubby"
  end

  test do
    assert_match "grubby", shell_output("#{bin}/grubby -v")
  end
end
