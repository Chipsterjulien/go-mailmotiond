package main

import (
	"bitbucket.org/zombiezen/cardcpx/natsort"
	"crypto/tls"
	"errors"
	"github.com/scorredoira/email"
	"github.com/vaughan0/go-ini"
	"log"
	"net/smtp"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func conf_errors(ok *bool, str string) {
	if !*ok {
		log.Fatal(str, "is not define in conf file !")
	}
}

func find_picture(picture_path string, _ bool) []string {
	types := []string{"*.jpg", "*.JPG", "*.ppm", "*.PPM"}
	pictures_list := make([]string, 0)

	for _, ext := range types {
		a, err := filepath.Glob(filepath.Join(picture_path, ext))
		if err != nil {
			log.Println(err, " in \"find_picture\" func")
		}

		for _, f := range a {
			pictures_list = append(pictures_list, f)
		}
	}

	natsort.Strings(pictures_list)

	return pictures_list
}

func load_conf(conf_file string) ini.File {
	file, err := ini.LoadFile(conf_file)
	if err != nil {
		log.Fatal(err, " in \"load_conf\" func")
	}

	return file
}

func remove_picture(pictures_list *[]string) {
	for _, pic := range *pictures_list {
		os.Remove(pic)
	}
}

func send_email(picture *string, conf_obj *ini.File) error {
	m := email.NewMessage("Détection d'un mouvement suspect",
		"")
	m.From, _ = conf_obj.Get("Mail", "login")
	tmp, _ := conf_obj.Get("Mail", "smtp")
	m.From = m.From + "@" + strings.Join(strings.Split(tmp, ".")[1:], ".")
	tmp, _ = conf_obj.Get("Mail", "send to")
	m.To = make([]string, 0)
	
	if strings.Contains(tmp, ",") {
		for _, i := range strings.Split(tmp, ",") {
			m.To = append(m.To, strings.Replace(i, " ", "", -1))
		}
	} else if strings.Contains(tmp, " ") {
		for _, i := range strings.Split(tmp, " ") {
			if i != "" {
				m.To = append(m.To, i)
			}
		}
	} else {
		m.To = []string{tmp}
	}

	err := m.Attach(*picture)
	if err != nil {
		log.Println(err, " in \"send_email\" func")
		return errors.New("Can't attach picture")
	}

	host, _ := conf_obj.Get("Mail", "smtp")
	p, _ := conf_obj.Get("Mail", "port")
	login, _ := conf_obj.Get("Mail", "login")
	password, _ := conf_obj.Get("Mail", "password")

	port, _ := strconv.Atoi(p)
	servername := host + ":" + strconv.Itoa(port)
	auth := smtp.PlainAuth("", login, password, host)

	if port == 25 || port == 587 {
		err := email.Send(servername, auth, m)
		if err != nil {
			log.Println(err)
			return errors.New("Unable to send mail")
		}
	} else if port == 465 {
		tlsconfig := &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         host,
		}

		conn, err := tls.Dial("tcp", servername, tlsconfig)
		if err != nil {
			log.Println(err)
			return errors.New("Unable to dial with server")
		}

		c, err := smtp.NewClient(conn, host)
		if err != nil {
			log.Println(err)
			return errors.New("Unable to create a client to send mail")
		}

		// Authentify
		if err = c.Auth(auth); err != nil {
			log.Println(err)
			return errors.New("Unable to identify")
		}

		// Define the emmiter
		if err = c.Mail(m.From); err != nil {
			log.Println(err)
			return errors.New("Unable to define emitter")
		}

		// Define the recipients
		for _, addr := range m.To {
    		if err = c.Rcpt(addr); err != nil {
    			log.Println(err)
    			return errors.New("Unable to define the recepter")
    		}
    	}

		data, err := c.Data()
		if err != nil {
			log.Println(err)
			return errors.New("Unable to create a buffer to write data")
		}

		_, err = data.Write(m.Bytes())
		if err != nil {
			log.Println(err)
			return errors.New("Unable to write data on server")
		}

		if err = data.Close(); err != nil {
			log.Println(err)
			return errors.New("Unable to close the buffer")
		}
		c.Quit()

	} else {
		log.Fatal("\"", port, "\"", " isn't a valide port in conf file")
	}
	
	return nil
}

func setup_logging(f string) {
	fd, err := os.OpenFile(f, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}

	log.SetOutput(fd)
}

func sleeping(t string, _ bool) {
	ts, _ := strconv.Atoi(t)
	time.Sleep(time.Second * time.Duration(ts))
}

func test_conf(conf_obj *ini.File) {
	c_def := []string{"sleep starting", "sleep time", "picture path",
		"as daemon"}
	c_mail := []string{"smtp", "port", "login", "password", "send to"}

	for _, val := range c_def {
		_, ok := conf_obj.Get("Default", val)
		conf_errors(&ok, val)
	}

	for _, val := range c_mail {
		_, ok := conf_obj.Get("Mail", val)
		conf_errors(&ok, val)
	}
}

func main() {
	setup_logging("/var/log/mailmotiond/main.log")
	conf_obj := load_conf("/etc/go-mailmotiond/mailmotiond.conf")
	test_conf(&conf_obj)

	d, _ := conf_obj.Get("Default", "as daemon")
	if daemon, _ := strconv.ParseBool(d); !daemon {
		// sleep before starting
		sleeping(conf_obj.Get("Default", "sleep starting"))
	}

	// Remove pictures before starting
	pictures_list := find_picture(conf_obj.Get("Default", "picture path"))
	remove_picture(&pictures_list)

	for {
		pictures_list := find_picture(conf_obj.Get("Default", "picture path"))
		for _, picture := range pictures_list {
			if err := send_email(&picture, &conf_obj); err == nil {
				// remove picture if it was sent
				remove_picture(&[]string{picture})
			} else {
				sleeping("20", false)
			}
		}
		sleeping(conf_obj.Get("Default", "sleep time"))
	}
}
