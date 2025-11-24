package rest

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"log/slog"
	"net/http"
	"review-assigner/core"
)

type Handler struct {
	service *core.Service
	log     *slog.Logger
}

func NewHandler(service *core.Service, log *slog.Logger) http.Handler {
	h := &Handler{
		service: service,
		log:     log,
	}

	router := mux.NewRouter()
	router.HandleFunc("/team/add", h.CreateTeam).Methods("POST")
	router.HandleFunc("/team/get", h.GetTeam).Methods("GET")
	router.HandleFunc("/users/setIsActive", h.SetUserActive).Methods("POST")
	router.HandleFunc("/users/getReview", h.GetUserReviews).Methods("GET")
	router.HandleFunc("/pullRequest/create", h.CreatePullRequest).Methods("POST")
	router.HandleFunc("/pullRequest/merge", h.MergePullRequest).Methods("POST")
	router.HandleFunc("/pullRequest/reassign", h.ReassignPullRequest).Methods("POST")
	router.HandleFunc("/stats", h.GetStats).Methods("GET")

	return router
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

func (h *Handler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	var req AddTeamRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON format")
		return
	}

	team := core.Team{
		TeamName: req.TeamName,
	}

	for _, member := range req.Members {
		team.Members = append(team.Members, core.TeamMember{
			UserID:   member.UserID,
			Username: member.Username,
			IsActive: member.IsActive,
		})
	}

	createdTeam, err := h.service.CreateTeam(r.Context(), team)
	if err != nil {
		if errors.Is(err, core.ErrTeamAlreadyExists) {
			writeError(w, http.StatusBadRequest, "TEAM_EXISTS", err.Error())
			return
		}
		fmt.Printf("DEBUG ERROR: %v\n", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create team")
		return
	}

	response := AddTeamResponse{
		Team: toTeamResponse(createdTeam),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func toTeamResponse(team core.Team) TeamResponse {
	response := TeamResponse{
		TeamName: team.TeamName,
	}

	for _, member := range team.Members {
		response.Members = append(response.Members, TeamMemberDTO{
			UserID:   member.UserID,
			Username: member.Username,
			IsActive: member.IsActive,
		})
	}

	return response
}

func (h *Handler) GetTeam(w http.ResponseWriter, r *http.Request) {
	teamName := r.URL.Query().Get("team_name")
	if teamName == "" {
		writeError(w, http.StatusBadRequest, "MISSING_PARAMETER", "team_name parameter is required")
		return
	}

	team, err := h.service.GetTeam(r.Context(), teamName)
	if err != nil {
		if errors.Is(err, core.ErrTeamNotFound) {
			writeError(w, http.StatusNotFound, "TEAM_NOT_FOUND", err.Error())
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := GetTeamResponse{
		Team: toTeamResponse(team),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) SetUserActive(w http.ResponseWriter, r *http.Request) {
	var req SetUserActiveRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON format")
		return
	}

	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, "MISSING_FIELD", "user_id is required")
		return
	}

	user, err := h.service.IsActive(r.Context(), req.UserID, req.IsActive)
	if err != nil {
		if errors.Is(err, core.ErrUserNotFound) {
			writeError(w, http.StatusNotFound, "USER_NOT_FOUND", err.Error())
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := SetUserActiveResponse{
		User: toUserResponse(user),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func toUserResponse(user core.User) UserResponse {
	return UserResponse{
		UserID:   user.UserID,
		Username: user.Username,
		TeamName: user.TeamName,
		IsActive: user.IsActive,
	}
}

func (h *Handler) CreatePullRequest(w http.ResponseWriter, r *http.Request) {
	var req CreatePRRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON format")
		return
	}

	if req.PullRequestID == "" || req.PullRequestName == "" || req.AuthorID == "" {
		writeError(w, http.StatusBadRequest, "MISSING_FIELDS", "pull_request_id, pull_request_name and author_id are required")
		return
	}

	pr, err := h.service.CreatePR(r.Context(), core.PullRequest{
		PullRequestID:   req.PullRequestID,
		PullRequestName: req.PullRequestName,
		AuthorID:        req.AuthorID,
	})
	if err != nil {
		switch {
		case errors.Is(err, core.ErrPRAAlreadyExists):
			writeError(w, http.StatusConflict, "PR_EXISTS", err.Error())
		case errors.Is(err, core.ErrUserNotFound):
			writeError(w, http.StatusNotFound, "AUTHOR_NOT_FOUND", err.Error())
		case errors.Is(err, core.ErrTeamNotFound):
			writeError(w, http.StatusNotFound, "TEAM_NOT_FOUND", err.Error())
		case errors.Is(err, core.ErrNotEnoughReviewers):
			writeError(w, http.StatusConflict, "NOT_ENOUGH_REVIEWERS", err.Error())
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	response := CreatePRResponse{
		PR: toPRResponse(pr),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func toPRResponse(pr core.PullRequest) PRResponse {
	return PRResponse{
		PullRequestID:     pr.PullRequestID,
		PullRequestName:   pr.PullRequestName,
		AuthorID:          pr.AuthorID,
		Status:            string(pr.Status),
		AssignedReviewers: pr.AssignedReviewers,
	}
}

func (h *Handler) MergePullRequest(w http.ResponseWriter, r *http.Request) {
	var req MergePRRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON format")
		return
	}

	if req.PullRequestID == "" {
		writeError(w, http.StatusBadRequest, "MISSING_FIELD", "pull_request_id is required")
		return
	}

	pr, err := h.service.Merged(r.Context(), req.PullRequestID)
	if err != nil {
		if errors.Is(err, core.ErrPRNotFound) {
			writeError(w, http.StatusNotFound, "PR_NOT_FOUND", err.Error())
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := MergePRResponse{
		PR: toPRResponse(pr),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
func (h *Handler) ReassignPullRequest(w http.ResponseWriter, r *http.Request) {
	var req ReassignReviewer

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON format")
		return
	}

	if req.PullRequestID == "" || req.OldUserID == "" {
		writeError(w, http.StatusBadRequest, "MISSING_FIELDS", "pull_request_id and old_user_id are required")
		return
	}

	coreReq := core.ReassignReviewer{
		PRId:   req.PullRequestID,
		UserID: req.OldUserID,
	}

	result, newReviewer, err := h.service.Reassign(r.Context(), coreReq)
	if err != nil {
		switch {
		case errors.Is(err, core.ErrPRNotFound):
			writeError(w, http.StatusNotFound, "PR_NOT_FOUND", err.Error())
		case errors.Is(err, core.ErrUserNotFound):
			writeError(w, http.StatusNotFound, "USER_NOT_FOUND", err.Error())
		case errors.Is(err, core.ErrPRAlreadyMerged):
			writeError(w, http.StatusConflict, "PR_MERGED", "can not reassign on merged PR")
		case errors.Is(err, core.ErrReviewerNotAssigned):
			writeError(w, http.StatusConflict, "NOT_ASSIGNED", "reviewer is not assigned to this PR ")
		case errors.Is(err, core.ErrNoReplacementCandidate):
			writeError(w, http.StatusConflict, "NO_CANDIDATE", "no active replacement candidate in team")
		default:
			// Используем единообразную функцию для ошибок
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		}
		return
	}

	response := ReassignPRResponse{
		PR:         toPRResponse(result),
		ReplacedBy: newReviewer,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) GetUserReviews(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "MISSING_PARAMETER", "user_id parameter is required")
		return
	}

	result, err := h.service.GetReview(r.Context(), userID)
	if err != nil {
		if errors.Is(err, core.ErrUserNotFound) {
			writeError(w, http.StatusNotFound, "USER_NOT_FOUND", err.Error())
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := GetUserReviewsResponse{
		UserID:       result.UserID,
		PullRequests: toPRShortResponses(result.PullRequest),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func toPRShortResponses(prs []core.PullRequest) []PRShortResponse {
	responses := make([]PRShortResponse, 0, len(prs))
	for _, pr := range prs {
		responses = append(responses, PRShortResponse{
			PullRequestID:   pr.PullRequestID,
			PullRequestName: pr.PullRequestName,
			AuthorID:        pr.AuthorID,
			Status:          string(pr.Status),
		})

	}
	return responses
}

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.GetStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "STATS_ERROR", "Failed to get statistics")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stats": stats,
	})
}
