package push

import (
	"context"
	"fmt"
	"time"

	"github.com/wargasipil/hai-tukang/internal/accounts"
	"github.com/wargasipil/hai-tukang/internal/visits"
)

// VisitNotifier tells people about visit events (visits.Notifier). It
// sends in the background, so a slow push service never holds up a change.
type VisitNotifier struct {
	Push *Service
}

func (n VisitNotifier) Notify(ctx context.Context, v visits.Visit, e visits.Event) {
	recipient, compose := visitMessage(v, e)
	if recipient == 0 {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		n.Push.Notify(ctx, recipient, compose)
	}()
}

// visitMessage is who to tell about e, and what, in a language and zone.
func visitMessage(v visits.Visit, e visits.Event) (int64, func(string, *time.Location) Message) {
	customer := party{v.CustomerID, v.CustomerName}
	repairman := party{v.RepairmanID, v.RepairmanName}
	proposer, other := customer, repairman
	if v.ProposedBy == accounts.RoleRepairman {
		proposer, other = repairman, customer
	}
	tag := fmt.Sprintf("visit-%d-%s", v.ID, e)
	message := func(title, body string) Message { return Message{Title: title, Body: body, URL: "/", Tag: tag} }
	switch e {
	case visits.EventProposed:
		return other.id, func(language string, zone *time.Location) Message {
			w := window(language, zone, v)
			if language == "en" {
				return message("Visit proposed by "+proposer.name, w+". Open Hai Tukang to accept or propose another time.")
			}
			return message("Usulan kunjungan dari "+proposer.name, w+". Buka Hai Tukang untuk menerima atau mengusulkan waktu lain.")
		}
	case visits.EventConfirmed:
		return proposer.id, func(language string, zone *time.Location) Message {
			w := window(language, zone, v)
			if language == "en" {
				return message("Visit confirmed", other.name+" accepted "+w+".")
			}
			return message("Kunjungan terkonfirmasi", other.name+" menerima "+w+".")
		}
	case visits.EventOverdue:
		return customer.id, func(language string, zone *time.Location) Message {
			w := window(language, zone, v)
			if language == "en" {
				return message("Has the repairman arrived?", "The "+w+" window ended 30 minutes ago. If "+repairman.name+" did not come, you can report it.")
			}
			return message("Tukang sudah tiba?", "Jadwal "+w+" sudah lewat 30 menit. Jika "+repairman.name+" tidak datang, Anda bisa melaporkannya.")
		}
	case visits.EventNoShowReported:
		return repairman.id, func(language string, zone *time.Location) Message {
			w := window(language, zone, v)
			if language == "en" {
				return message(customer.name+" reported that you did not come", "Visit "+w+". If you came, dispute it within 24 hours in Hai Tukang.")
			}
			return message(customer.name+" melaporkan Anda tidak datang", "Jadwal "+w+". Jika Anda datang, bantah dalam 24 jam di Hai Tukang.")
		}
	case visits.EventAbsentReported:
		return customer.id, func(language string, zone *time.Location) Message {
			w := window(language, zone, v)
			if language == "en" {
				return message(repairman.name+" reported that you were not at home", "Visit "+w+". If you were at home, dispute it within 24 hours in Hai Tukang.")
			}
			return message(repairman.name+" melaporkan Anda tidak ada di rumah", "Jadwal "+w+". Jika Anda ada di rumah, bantah dalam 24 jam di Hai Tukang.")
		}
	}
	return 0, nil
}

type party struct {
	id   int64
	name string
}

var (
	idDays   = [...]string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}
	idMonths = [...]string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}
)

// window writes v's window in a language and zone: "Sabtu 7 Nov, 09.00–11.00".
func window(language string, zone *time.Location, v visits.Visit) string {
	start, end := v.Start.In(zone), v.End.In(zone)
	if language == "en" {
		return fmt.Sprintf("%s %d %s, %s–%s", start.Format("Mon"), start.Day(), start.Format("Jan"), start.Format("15:04"), end.Format("15:04"))
	}
	return fmt.Sprintf("%s %d %s, %s–%s", idDays[start.Weekday()], start.Day(), idMonths[start.Month()-1], start.Format("15.04"), end.Format("15.04"))
}
