package main

import (
	"archive/zip"
	"log"
	"os"

	k "github.com/mcluseau/kubeclient"
)

func genKeyCommand() *Command {
	cmd := &Command{
		Use: "gen-key",
		Run: genKeyRun,
	}

	flags := cmd.Flags()
	flags.StringVar(&serverName, "server-name", "kgate", "The server name, for the certificate")

	return cmd
}

func genKeyRun(cmd *Command, args []string) {
	secretCA = serverName + "-ca"

	secCA, err := k.Client().CoreV1().Secrets(namespace).Get(secretCA, getOpts)
	if err != nil {
		log.Fatal(err)
	}

	sec := getOrCreateTLS(serverName+"-client", func() ([]byte, []byte) {
		key, keyPEM := PrivateKeyPEM()
		crtPEM := HostCertificatePEM(secCA.Data, 1, key, "client")
		return keyPEM, crtPEM
	})

	zipFile := serverName + "-client-config.zip"

	out, err := os.Create(zipFile)
	if err != nil {
		log.Fatal(err)
	}

	defer out.Close()

	log.Print("Writing configuration to ", zipFile)

	zw := zip.NewWriter(out)

	for name, data := range map[string][]byte{
		"url":         []byte("ws://" + serverName + "." + namespace + ".dev.isi.nc:80"),
		"server-name": []byte(serverName),
		"ca.crt":      secCA.Data["tls.crt"],
		"client.crt":  sec.Data["tls.crt"],
		"client.key":  sec.Data["tls.key"],
	} {
		f, err := zw.Create(name)
		if err != nil {
			log.Fatal(err)
		}

		if _, err := f.Write(data); err != nil {
			log.Fatal(err)
		}
	}

	if err := zw.Close(); err != nil {
		log.Fatal(err)
	}
}
